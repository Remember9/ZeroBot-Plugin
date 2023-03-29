package aths

import (
	"fmt"
	"github.com/FloatTech/AnimeAPI/nsfw"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
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
	engine.OnPrefixGroup([]string{"js", "记事"}).SetBlock(false).
		Handle(func(ctx *zero.Ctx) {
			fmt.Println("ctx.Event.RawMessage=")
			fmt.Println(ctx.Event.RawMessage)

			// 将CQ码中的图片URL替换为本地路径
			if zero.HasPicture(ctx) {
				var updatedRawMsg string
				urls := ctx.State["image_url"].([]string)
				segments := []message.MessageSegment{message.Text("收到图片", ctx.Event.RawMessage)}
				for _, url := range urls {
					// 消息中的图片下载下来存储到本地
					relPath, filename, err := downloadImage(url, ctx.Event.Sender.ID, 3)
					if err != nil {
						logrus.Info(fmt.Sprintf("图片%v下载失败", url))
						ctx.SendChain(message.Text(fmt.Sprintf("图片%v下载失败", url)))
						return
					}
					// 用图片本地存储路径替换CQ码中的URL
					fmt.Printf("relPath=%v, filename=%v, joined=%v\n", relPath, filename, filepath.Join(relPath, filename))
					updatedRawMsg = strings.Replace(ctx.Event.RawMessage, url, filepath.Join(relPath, filename), 1)
					segments = append(segments, message.Image(url))
				}
				ctx.SendChain(segments...)
				fmt.Printf("updatedRawMsg更新后=%v", updatedRawMsg)
				ctx.SendChain(message.Text(updatedRawMsg))
			}

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

func downloadImage(url string, senderId int64, maxRetry int) (string, string, error) {
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
		return "", "", fmt.Errorf("Failed to download image after %d retries. Error: %v", maxRetry, err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	// 获取文件扩展名
	ext, err := mimetype.DetectReader(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("Failed to detect image mimetype. Error: %v", err)
	}

	// 生成文件名
	filename := strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + uuid.New().String()[:8] + "." + ext.Extension()

	//// 获取当前工作目录
	//dir, err := os.Getwd()
	//if err != nil {
	//	logrus.Info("Failed to get working directory:", err)
	//	return
	//}
	//logrus.Info("Working directory:", dir)
	// 获取相对路径
	relPath := filepath.Join("data", strconv.FormatInt(senderId, 10), "images")
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		return "", "", fmt.Errorf("Failed to get absolute path. Error: %v", err)
	}

	// 创建目录
	err = os.MkdirAll(absPath, os.ModePerm)
	if err != nil {
		return "", "", fmt.Errorf("Failed to create directory. Error: %v", err)
	}

	// 将图片保存到本地
	filePath := filepath.Join(absPath, filename)
	err = os.WriteFile(filePath, body, 0666)
	if err != nil {
		return "", "", fmt.Errorf("Failed to save image. Error: %v", err)
	}
	fmt.Printf("Image saved as %s\n", filename)

	return relPath, filename, nil
}
