package aths

import (
	"bytes"
	"fmt"
	"github.com/FloatTech/AnimeAPI/nsfw"
	"github.com/FloatTech/ZeroBot-Plugin/plugin/aths/model"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const imageRootPath = "/var/bot/"
const hso = "https://gchat.qpic.cn/gchatpic_new//--4234EDEC5F147A4C319A41149D7E0EA9/0"

const (
	funcTypeNote = iota + 1
	funcTypeJishi
	funcTypeEnglish
	funcTypeVocabulary
	funcTypeBook
	funcTypeTodayLearn
	funcTypeAutoReview
	funcTypeAutoRemind
)

const (
	noReview = 0
	review   = 1
)

func init() {
	// 注册引擎
	engine := control.Register("js", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "艾涛浩斯记事本",
		Help:             "- js+[要发送的图片]",
	})
	// 群文件
	engine.OnPrefixGroup([]string{"wjlb", "文件列表"}).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		println("文件列表")
		getFileURLbyFileName(ctx, "")
		fileList := getGroupFileList(ctx)
		files := strings.Join(fileList, "\n")
		println(files)
		ctx.SendChain(message.Text(files))
	})

	// 查群文件
	engine.OnPrefixGroup([]string{"wjurl"}).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		nameList := []string{"地质.jpg", "公式.jpg", "dizhi.jpg"}
		for _, name := range nameList {
			_, url := getFileURLbyFileName(ctx, name)
			ctx.SendChain(message.Text(name))
			ctx.SendChain(message.Image(url))
			ctx.SendChain(message.Image("https://gchat.qpic.cn/gchatpic_new/1506445901/718853660-2934326276-56A46A5603EDC69EE50A4358D4C14691/0?term=255&amp;is_origin=0"))
		}

		println(getFileURLbyFileName(ctx, "地质.jpg"))
		println(getFileURLbyFileName(ctx, "公式.jpg"))
		println(getFileURLbyFileName(ctx, "dizhi.jpg"))
	})

	// 保存收到的图文
	jsPrefixes := []string{"js", "记事"}
	engine.OnPrefixGroup(jsPrefixes).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		qqNumber := ctx.Event.Sender.ID
		// 将CQ码中的图片URL替换为本地路径
		for _, elem := range ctx.Event.Message {
			if elem.Type == "image" {
				if url := elem.Data["url"]; url != "" {
					// 消息中的图片下载下来存储到本地
					filename, err := downloadImage(ctx, url, qqNumber, 3)
					elem.Data["local_name"] = filename
					if err != nil {
						logrus.Info(fmt.Sprintf("图片%v下载失败", url))
						return
					}
				}
			}
		}

		// 去除命令前缀
		finalCQCode := ctx.Event.Message.CQCode()
		for _, prefix := range jsPrefixes {
			if strings.HasPrefix(finalCQCode, prefix) {
				finalCQCode = strings.TrimLeft(finalCQCode[len(prefix):], " ")
				break
			}
		}
		logrus.Info("最终要入库的CQ码消息=", finalCQCode)
		note := &model.Note{
			QQNumber: strconv.FormatInt(qqNumber, 10),
			Type:     funcTypeJishi,
			IsReview: 0,
			Content:  finalCQCode,
			CDate:    time.Now(),
		}
		if err := GetDB().Create(note).Error; err != nil {
			logrus.Errorf("笔记插入失败, note=%v, err=%s", *note, err.Error())
			ctx.SendChain(message.Text("笔记插入失败"))
			return
		}
		ctx.SendChain(message.Text(finalCQCode))
	})

	// 发送图文
	ckjsPrefix := []string{"ckjs", "查看记事"}
	engine.OnPrefixGroup(ckjsPrefix).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		params := removePrefix(ctx.Event.RawMessage, ckjsPrefix)
		qqNumber := ctx.Event.Sender.ID
		s := "导入\n[CQ:image,file=48f1dce3e0a2391d8ef4cfe3e2a3609a.image,url=https://c2cpicdw.qpic.cn/offpic_new/164212720//164212720-976498784-48F1DCE3E0A2391D8EF4CFE3E2A3609A/0?term=255&amp;is_origin=0,local_name=2023T0329T181632.722262.png]\n依赖"
		m := message.ParseMessageFromString(s)
		notes, err := queryNotes(strconv.FormatInt(qqNumber, 10), funcTypeJishi, params, 10)
		logrus.Infof("params=%v, notes=%v", params, notes)
		if err != nil {
			logrus.Errorf("查询笔记失败：%v", err)
		}
		var mList []message.Message
		var segList []message.MessageSegment
		for _, note := range notes {
			m := message.ParseMessageFromString(note.Content + "\n")
			mList = append(mList, m)
			segList = append(segList, m...)
		}
		// 将CQ码中的图片URL替换为本地路径
		for _, elem := range m {
			if elem.Type == "image" {
				// 检查CQ码中是否存储了图片的本地文件名
				if localName, ok := elem.Data["local_name"]; ok {
					// 获取相对路径
					relPath := filepath.Join("data", strconv.FormatInt(ctx.Event.Sender.ID, 10), "images", localName)
					absPath, err := filepath.Abs(relPath)
					if err != nil {
						logrus.Info(fmt.Errorf("failed to get absolute path: %v. Error: %v", absPath, err))
						return
					}

					// 读取本地二进制图片, 路径分隔符使用Unix格式
					absPath = filepath.ToSlash(absPath)
					imageContent, err := os.ReadFile(absPath)
					if err != nil {
						logrus.Info(fmt.Errorf("failed to read absolute path: %v. Error: %v", absPath, err))
						return
					}

					// 发送时使用本地图片
					elem = message.ImageBytes(imageContent)
				}
			}
			//println(elem.CQCode())
			//ctx.SendChain(elem)
		}
		// 把多个单独消息拼接成一条长消息
		var endMsg []message.MessageSegment
		for i, msg := range mList {
			if i != 0 { // 两条消息之间添加换行
				endMsg = append(endMsg, message.Text("\n"))
			}
			endMsg = append(endMsg, message.Text(i+1, ". ")) // 给每条笔记添加序号
			endMsg = append(endMsg, msg...)
		}
		ctx.SendChain(endMsg...)
	})
}

func autojudge(ctx *zero.Ctx, p *nsfw.Picture) {
	if p.Neutral > 0.3 {
		return
	}
	c := ""
	ctx.Send(message.ReplyWithMessage(ctx.Event.MessageID, message.Text(c, "\n"), message.Image(hso)))
}

// 设置伪装浏览器header
var headers = map[string]string{
	"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3",
}

func downloadImage(ctx *zero.Ctx, url string, senderId int64, maxRetry int) (string, error) {
	var err error
	var resp *http.Response
	for i := 0; i < maxRetry; i++ {
		client := &http.Client{}
		req, _ := http.NewRequest("GET", url, nil)
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		fmt.Printf("Retry %d times due to error: %v\n", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return "", fmt.Errorf("Failed to download image after %d retries. Error: %v", maxRetry, err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 获取文件扩展名
	_, imageFormat, err := image.DecodeConfig(bytes.NewBuffer(body))
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	// 生成文件名
	filename := time.Now().Format("2006-01-02T150405.000000") + "." + imageFormat

	// 获取相对路径
	relPath := filepath.Join("data", strconv.FormatInt(senderId, 10), "images")
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		return "", fmt.Errorf("Failed to get absolute path. Error: %v", err)
	}

	// 创建目录
	err = os.MkdirAll(absPath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("Failed to create directory. Error: %v", err)
	}

	// 将图片保存到本地
	filePath := filepath.Join(absPath, filename)
	err = os.WriteFile(filePath, body, 0666)
	if err != nil {
		return "", fmt.Errorf("Failed to save image. Error: %v", err)
	}
	fmt.Printf("Image saved as %s\n", filename)

	// 上传到群文件
	//ctx.UploadGroupFile(718853660, filePath, filename, "")
	files := ctx.GetThisGroupRootFiles(0)
	fmt.Printf("files=%v", files)
	upResp := ctx.UploadThisGroupFile(filePath, filename, "/131a5840-3c2f-4fe7-88bc-42029bd2d931")
	fmt.Printf("上传群文件结果：%v", upResp)

	return filename, nil
}

func queryNotes(userId string, noteType int8, keyword string, limit int) ([]model.Note, error) {
	db := GetDB()
	var notes []model.Note
	//queryDb := db.Select("id, content, note_url").Where(model.Note{QQNumber: userId, Type: noteType, IsDelete: 0}).Order("cdate desc")
	queryDb := db.Where(model.Note{QQNumber: userId, Type: noteType, IsDelete: 0}).Order("cdate desc")
	if keyword != "" {
		queryDb.Where("content like ?", "%"+keyword+"%")
	}
	if limit != -1 {
		queryDb.Limit(limit)
	}
	queryDb.Debug().Find(&notes)
	return notes, queryDb.Error
}

// 去除前缀
func removePrefix(message string, prefixes []string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
			message = strings.TrimSpace(message)
			return message
		}
	}
	return message
}

// 遍历群文件
func getFileURLbyFileName(ctx *zero.Ctx, fileName string) (fileSearchName, fileURL string) {
	filesOfGroup := ctx.GetThisGroupRootFiles(ctx.Event.GroupID)
	files := filesOfGroup.Get("files").Array()
	folders := filesOfGroup.Get("folders").Array()
	// 遍历当前目录的文件名
	if len(files) != 0 {
		for _, fileNameOflist := range files {
			if strings.Contains(fileNameOflist.Get("file_name").String(), fileName) {
				fileSearchName = fileNameOflist.Get("file_name").String()
				fileURL = ctx.GetThisGroupFileUrl(fileNameOflist.Get("busid").Int(), fileNameOflist.Get("file_id").String())
				return
			}
		}
	}
	// 遍历子文件夹
	if len(folders) != 0 {
		for _, folderNameOflist := range folders {
			folderID := folderNameOflist.Get("folder_id").String()
			fileSearchName, fileURL = getFileURLbyfolderID(ctx, fileName, folderID)
			if fileSearchName != "" {
				return
			}
		}
	}
	return
}

// 遍历群文件
func getGroupFileList(ctx *zero.Ctx) (fileList []string) {
	filesOfGroup := ctx.GetThisGroupRootFiles(ctx.Event.GroupID)
	files := filesOfGroup.Get("files").Array()
	//folders := filesOfGroup.Get("folders").Array()
	// 遍历当前目录的文件名

	var fileInfo string
	if len(files) != 0 {
		for i, fileNameOflist := range files {
			fileInfo = fmt.Sprintf("%d. filename=%s,busid=%s,file_id=%s",
				i+1,
				fileNameOflist.Get("file_name").String(),
				fileNameOflist.Get("busid"),
				fileNameOflist.Get("file_id").String(),
			)
			fileList = append(fileList, fileInfo)
		}
	}
	return
	// 遍历子文件夹
	//if len(folders) != 0 {
	//	for _, folderNameOflist := range folders {
	//		folderID := folderNameOflist.Get("folder_id").String()
	//		fileSearchName, fileURL = getFileURLbyfolderID(ctx, fileName, folderID)
	//		if fileSearchName != "" {
	//			return
	//		}
	//	}
	//}
	//return
}
func getFileURLbyfolderID(ctx *zero.Ctx, fileName, folderid string) (fileSearchName, fileURL string) {
	filesOfGroup := ctx.GetThisGroupFilesByFolder(folderid)
	files := filesOfGroup.Get("files").Array()
	folders := filesOfGroup.Get("folders").Array()
	// 遍历当前目录的文件名
	if len(files) != 0 {
		for _, fileNameOflist := range files {
			if strings.Contains(fileNameOflist.Get("file_name").String(), fileName) {
				fileSearchName = fileNameOflist.Get("file_name").String()
				fileURL = ctx.GetThisGroupFileUrl(fileNameOflist.Get("busid").Int(), fileNameOflist.Get("file_id").String())
				return
			}
		}
	}
	// 遍历子文件夹
	if len(folders) != 0 {
		for _, folderNameOflist := range folders {
			folderID := folderNameOflist.Get("folder_id").String()
			fileSearchName, fileURL = getFileURLbyfolderID(ctx, fileName, folderID)
			if fileSearchName != "" {
				return
			}
		}
	}
	return
}
