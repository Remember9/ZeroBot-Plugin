package aths

import (
	"bytes"
	"fmt"
	"github.com/FloatTech/AnimeAPI/nsfw"
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

func init() {
	engine := control.Register("js", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "艾涛浩斯记事本",
		Help:             "- js+[要发送的图片]",
	})
	// 保存收到的图文
	jsPrefixes := []string{"js", "记事"}
	engine.OnPrefixGroup(jsPrefixes).SetBlock(false).
		Handle(func(ctx *zero.Ctx) {
			fmt.Println(ctx.Event.RawMessage)
			println("ctx.Event.Message.CQCode()=", ctx.Event.Message.CQCode())

			// 将CQ码中的图片URL替换为本地路径
			for _, elem := range ctx.Event.Message {
				if elem.Type == "image" {
					if url := elem.Data["url"]; url != "" {
						// 消息中的图片下载下来存储到本地
						filename, err := downloadImage(url, ctx.Event.Sender.ID, 3)
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

			println("最终要入库的CQ码消息=", finalCQCode)
			ctx.SendChain(message.Text(message.EscapeCQCodeText(finalCQCode)))
			//ctx.SendChain()
			//if zero.HasPicture(ctx) {
			//	var updatedRawMsg string
			//	urls := ctx.State["image_url"].([]string)
			//	segments := []message.MessageSegment{message.Text("收到图片", ctx.Event.RawMessage)}
			//	for _, url := range urls {
			//		// 消息中的图片下载下来存储到本地
			//		relPath, filename, err := downloadImage(url, ctx.Event.Sender.ID, 3)
			//		if err != nil {
			//			logrus.Info(fmt.Sprintf("图片%v下载失败", url))
			//			ctx.SendChain(message.Text(fmt.Sprintf("图片%v下载失败", url)))
			//			return
			//		}
			//		// 用图片本地存储路径替换CQ码中的URL
			//		fmt.Printf("relPath=%v, filename=%v, joined=%v\n", filepath.ToSlash(relPath), filename, filepath.ToSlash(filepath.Join(relPath, filename)))
			//		updatedRawMsg = strings.Replace(ctx.Event.RawMessage, url, filepath.Join(relPath, filename), 1)
			//		segments = append(segments, message.Image(url))
			//	}
			//	ctx.SendChain(segments...)
			//	fmt.Printf("updatedRawMsg更新后=%v", updatedRawMsg)
			//	ctx.SendChain(message.Text(updatedRawMsg))
			//}

		})

	// 发送图文
	engine.OnPrefixGroup([]string{"ckjs", "查看记事"}).SetBlock(false).
		Handle(func(ctx *zero.Ctx) {
			s := "导入\n[CQ:image,file=48f1dce3e0a2391d8ef4cfe3e2a3609a.image,url=https://c2cpicdw.qpic.cn/offpic_new/164212720//164212720-976498784-48F1DCE3E0A2391D8EF4CFE3E2A3609A/0?term=255&amp;is_origin=0,local_name=2023T0329T181632.722262.png]\n依赖"
			m := message.ParseMessageFromString(s)
			//println(m)
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
				println(elem.CQCode())
				ctx.SendChain(elem)
			}

			//ctx.SendChain(m...)
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

func downloadImage(url string, senderId int64, maxRetry int) (string, error) {
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

	return filename, nil
}
