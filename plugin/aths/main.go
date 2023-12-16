package aths

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/FloatTech/ZeroBot-Plugin/plugin/aths/model"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/sirupsen/logrus"
	"github.com/syyongx/php2go"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
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
	funcTypeTopic
)

const (
	noReview = 0
	review   = 1
)

const (
	NoDeleted = 0
	Deleted   = 1
)

// var folderIdDict = map[int64]string{
//	164212720: "/131a5840-3c2f-4fe7-88bc-42029bd2d931",
// }

type userInfo struct {
	cmdType int
	idMap   map[int]uint
}

var userInfos map[int64]userInfo

// var topicIdMap map[int]string
// var topicNameMap map[string]int

type topicmap struct {
	id2Name map[int][]string
	name2Id map[string]int
}

var topicMap map[int64]topicmap

func StartServer() {
	type notify struct {
		Receiver     int64  `json:"receiver"`
		ReceiverType string `json:"receiver_type"`
		Message      string `json:"message"`
	}
	http.HandleFunc("/checkin", func(w http.ResponseWriter, r *http.Request) {
		// Set the content type for the response to JSON
		w.Header().Set("Content-Type", "application/json")

		// Parse the JSON request body
		var requestData notify
		if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
			http.Error(w, "Failed to parse JSON request", http.StatusBadRequest)
			return
		}
		logrus.Printf("requestData=%+v", requestData)
		// 获取一个在线能用的bot
		zero.RangeBot(func(id int64, c *zero.Ctx) bool {
			switch requestData.ReceiverType {
			case "group":
				c.SendGroupMessage(requestData.Receiver, message.Text(requestData.Message))
			case "private":
				c.SendPrivateMessage(requestData.Receiver, message.Text(requestData.Message))
			}
			return false
		})

		// Write the response back to the client
		_, _ = w.Write([]byte(`{"code": 0, "message": "success"}`))
	})

	// 启动HTTP服务器并监听端口
	port := "1642"
	fmt.Printf("Server listening on port %s...\n", port)
	err := http.ListenAndServe("0.0.0.0:"+port, nil)
	if err != nil {
		panic(err)
	}
}

func buildTopicMap() {
	topicIdMap := make(map[int][]string)
	topicNameMap := make(map[string]int)
	topicMap = make(map[int64]topicmap)
	db := GetDB()
	var reminds []model.Remind
	db.Model(&model.Remind{}).Select("DISTINCT topic_id, topic_name, qq_number").Find(&reminds)
	for _, remind := range reminds {
		// 排除无意义数据
		if remind.TopicName == "" || remind.QQNumber == "" {
			continue
		}
		// 建立topic_name和topic_id的映射关系
		topicNames := strings.Split(remind.TopicName, ",")
		for _, topicName := range topicNames {
			topicNameMap[strings.TrimSpace(topicName)] = remind.TopicId
		}
		// 建立topic_id和topic_name的映射关系
		topicIdMap[remind.TopicId] = topicNames
		// convert remind.QQNumber from string to int64
		qqNumber, err := strconv.ParseInt(remind.QQNumber, 10, 64)
		if err != nil {
			continue
		}
		topicMap[qqNumber] = topicmap{id2Name: topicIdMap, name2Id: topicNameMap}
	}
}

func init() {
	userInfos = make(map[int64]userInfo)
	// 注册艾涛浩斯引擎
	engine := control.Register("js", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "艾涛浩斯记事本",
		Help:             "- js+[要发送的图片]",
	})

	// 构建话题id与name的映射
	go func() {
		time.Sleep(2 * time.Second)
		buildTopicMap()
	}()

	// 开启一个定时器，定时查询表中是否有到时间的提醒
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		time.Sleep(5 * time.Second)
		var ctx *zero.Ctx
		// 查询待提醒事件
		for range ticker.C {
			// 获取一个在线能用的bot
			zero.RangeBot(func(id int64, c *zero.Ctx) bool {
				ctx = c
				return false
			})
			if ctx == nil {
				logrus.Errorln("定时器zero.Ctx==nil, 未获取到机器人实例，无法发送消息")
				continue
			}
			CheckReminderEvents(ctx)
		}
	}()

	// go func() {
	// 	StartServer()
	// }()

	// 添加新的定时提醒
	engine.OnMessage().SetBlock(false).Handle(func(ctx *zero.Ctx) {
		if !strings.Contains(ctx.Event.RawMessage, "提醒") && !strings.Contains(ctx.Event.RawMessage, "叫") {
			return
		}

		nextRemind, err := remindResolve(ctx.Event.RawMessage)
		if err != nil {
			return
		}

		if nextRemind.Time.Before(time.Now()) || (nextRemind.Time.Sub(time.Now()) < 1*time.Minute && nextRemind.Time.Minute() <= time.Now().Minute()) {
			logrus.Info("解析失败：计划提醒时间必须在一分钟之后")
			ctx.SendChain(message.Text("计划提醒时间必须在一分钟之后"))
			return
		}
		var qqNumber string
		if nextRemind.RemindQQ != "" {
			qqNumber = nextRemind.RemindQQ
		} else {
			qqNumber = strconv.FormatInt(ctx.Event.Sender.ID, 10)
		}
		groupNumber := strconv.FormatInt(ctx.Event.GroupID, 10)

		data := map[string]interface{}{
			"next_remind_time": nextRemind.Time.Format("2006-01-02 15:04:05"),
			"content":          nextRemind.Content,
			"qq_number":        qqNumber,
			"group_number":     groupNumber,
			"status":           TaskStatusOn,
			"type":             TaskTypeTodo,
			"cdate":            time.Now().Format("2006-01-02 15:04:05"),
			"remind_rule":      nextRemind.RemindRule,
			"is_repeat":        map[bool]int{false: 0, true: 1}[nextRemind.IsRepeat],
		}

		logrus.Infof("本次定时提醒要入库数据%v", data)

		dao := GetDB()

		if err := dao.Debug().Model(&model.Remind{}).Create(data).Error; err == nil {
			logrus.Infof("新建定时提醒成功:%s", ctx.Event.RawMessage)
			name := "你"
			fmt.Printf("nextRemind.RemindQQ=%v, qqNumber=%v\n", nextRemind.RemindQQ, qqNumber)
			if nextRemind.RemindQQ != "" {
				name = "这个傻逼"
			}
			ctx.SendChain(message.Text(fmt.Sprintf("好的，%s我会提醒%s%s", data["remind_rule"], name, data["content"])))
		} else {
			logrus.Errorf("新建定时提醒失败:%v", err)
			ctx.SendChain(message.Text("新建定时提醒失败"))
		}

	})

	// 保存收到的图文
	jsPrefixes := []string{"js", "记事"}
	engine.OnPrefixGroup(jsPrefixes).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		qqNumber := ctx.Event.Sender.ID
		for _, elem := range ctx.Event.Message {
			switch elem.Type {
			case "image":
				if url := elem.Data["url"]; url != "" {
					filename, err := downloadImage(ctx, url, qqNumber, 3)
					if err != nil {
						logrus.Info(fmt.Sprintf("图片%v下载失败", url))
						return
					}
					elem.Data["local_name"] = filename
				}
			}
		}
		// 修改后的消息重新生成CQ码
		finalCQCode := ctx.Event.Message.CQCode()

		// 去除命令前缀
		for _, prefix := range jsPrefixes {
			if strings.HasPrefix(finalCQCode, prefix) {
				finalCQCode = strings.TrimPrefix(finalCQCode, prefix)
				finalCQCode = strings.TrimLeft(finalCQCode, " ")
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
		err := GetDB().Create(note).Error
		if err != nil {
			logrus.Errorf("笔记插入失败, note=%v, err=%s", *note, err.Error())
			ctx.SendChain(message.Text("笔记插入失败"))
			return
		}

		ctx.Send(ctx.Event.Message)
	})

	// 给话题里添加内容
	engine.OnMessage().SetBlock(false).Handle(func(ctx *zero.Ctx) {
		// 判断是否为两个参数
		fields := strings.SplitN(strings.TrimSpace(ctx.Event.RawMessage), " ", 2)
		if len(fields) < 2 {
			return
		}
		qqNumber := ctx.Event.Sender.ID
		topicName := fields[0]
		topicId, ok := topicMap[qqNumber].name2Id[topicName]
		if !ok {
			return
		}
		for _, elem := range ctx.Event.Message {
			switch elem.Type {
			case "image":
				if url := elem.Data["url"]; url != "" {
					filename, err := downloadImage(ctx, url, qqNumber, 3)
					if err != nil {
						logrus.Info(fmt.Sprintf("图片%v下载失败", url))
						return
					}
					elem.Data["local_name"] = filename
				}
			}
		}
		// 修改后的消息重新生成CQ码
		finalCQCode := ctx.Event.Message.CQCode()

		// 去除话题名
		finalCQCode = strings.TrimPrefix(finalCQCode, topicName)
		finalCQCode = strings.TrimLeft(finalCQCode, " ")
		logrus.Info("最终要入库的CQ码消息=", finalCQCode)

		note := &model.Note{
			QQNumber: strconv.FormatInt(qqNumber, 10),
			Type:     topicId,
			IsReview: 0,
			Content:  finalCQCode,
			CDate:    time.Now(),
		}
		err := GetDB().Create(note).Error
		if err != nil {
			logrus.Errorf("话题内容插入失败, note=%v, err=%s", *note, err.Error())
			ctx.SendChain(message.Text("话题内容插入失败"))
			return
		}

		ctx.Send(ctx.Event.Message)
	})

	// 新建话题
	engine.OnPrefixGroup([]string{"xjht", "新建话题", "cjht", "创建话题"}).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		args := ctx.State["args"].(string)
		qqNumber := ctx.Event.Sender.ID
		if args == "" {
			ctx.SendChain(message.Text("请指定话题名，例：新建话题名 算命"))
			return
		}
		_, exist := topicMap[qqNumber].name2Id[args]
		if exist {
			ctx.SendChain(message.Text(fmt.Sprintf("话题“%s“已存在，不允许存在相同名称的话题", args)))
			return
		}

		GroupID := ctx.Event.GroupID
		var maxTopicId int
		topicIdStart := 100
		db := GetDB()
		var err error
		if err = db.Model(&model.Remind{}).Select("COALESCE(MAX(topic_id), 0)").Scan(&maxTopicId).Error; err != nil {
			logrus.Errorf("查询最大topic_id失败, err=%s", err.Error())
			ctx.SendChain(message.Text("查询最大topic_id失败"))
			return
		}
		if maxTopicId == 0 {
			maxTopicId = topicIdStart
		}
		newTopicId := maxTopicId + 1 // 新话题ID
		note := &model.Remind{
			QQNumber:    strconv.FormatInt(qqNumber, 10),
			Type:        TaskTypeTopic,
			GroupNumber: strconv.FormatInt(GroupID, 10),
			Status:      TaskStatusOff, // 刚创建时默认不加入自动提醒
			CDate:       time.Now(),
			TopicId:     newTopicId,
			TopicName:   args,
		}
		if err = db.Create(note).Error; err != nil {
			logrus.Errorf("话题插入失败, note=%v, err=%s", *note, err.Error())
			ctx.SendChain(message.Text("话题插入失败"))
			return
		}
		// 构建话题id与name的映射
		buildTopicMap()

		ctx.SendChain(message.Text(fmt.Sprintf("新建话题：%s成功", args)))
	})

	// 为话题添加别名
	engine.OnPrefixGroup([]string{"htbm", "话题别名", "bm", "别名", "tjbm", "添加别名"}).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		args := ctx.State["args"].(string)
		qqNumber := ctx.Event.Sender.ID
		// split args to 2 parts by space
		parts := strings.SplitN(args, " ", 2)
		if len(parts) < 2 {
			return
		}
		oldTopicName := strings.TrimSpace(parts[0])
		topicId, exist := topicMap[qqNumber].name2Id[oldTopicName]
		if exist {
			ctx.SendChain(message.Text(fmt.Sprintf("话题”%s”已存在，请起一个其他别名", oldTopicName)))
			return
		}
		db := GetDB()
		newTopicName := strings.TrimSpace(parts[1])
		// 新别名添加到该话题别名列表
		topicMap[qqNumber].id2Name[topicId] = append(topicMap[qqNumber].id2Name[topicId], newTopicName)
		aliaNames := strings.Join(topicMap[qqNumber].id2Name[topicId], ",")
		if err := db.Where("topic_id = ?", topicId).Updates(model.Remind{TopicName: aliaNames}).Error; err != nil {
			logrus.Errorf("话题别名%s新增失败, err=%s", newTopicName, err.Error())
			ctx.SendChain(message.Text(fmt.Sprintf("话题别名%s新增失败, err=%s", newTopicName, err.Error())))
			return
		}
		// 构建话题id与name的映射
		buildTopicMap()
		// 该topic现有全部别名
		ctx.SendChain(message.Text(fmt.Sprintf("成功给话题“%s“添加别名：%s, 等价别名有：%s", oldTopicName, newTopicName, aliaNames)))
	})

	// 查看单个话题内容
	engine.OnMessage().SetBlock(false).Handle(func(ctx *zero.Ctx) {
		// 判断ctx.Event.RawMessage trim后是否为topicNameMap中的一个key'
		trimmedName := strings.TrimSpace(ctx.Event.RawMessage)
		qqNumber := ctx.Event.Sender.ID
		topicId, ok := topicMap[qqNumber].name2Id[trimmedName]
		if !ok {
			return
		}

		notes, err := queryNotes(strconv.FormatInt(qqNumber, 10), topicId, "", -1)
		logrus.Infof("notes=%v", notes)
		if err != nil {
			logrus.Errorf("查询笔记失败：%v", err)
		}
		if len(notes) == 0 {
			logrus.Info("话题数量为0")
			ctx.SendChain(message.Text("话题数量为0"))
			return
		}
		var mList []message.Message
		var segList []message.MessageSegment
		idMap := map[int]uint{}
		for i, note := range notes {
			m := message.ParseMessageFromString(note.Content + "\n")
			mList = append(mList, m)
			segList = append(segList, m...)
			idMap[i+1] = note.ID // 记事列表id映射关系
		}
		// 保存该用户话题列表id映射关系
		userInfos[qqNumber] = userInfo{cmdType: funcTypeTopic, idMap: idMap}

		// 把多个单独消息拼接成一条长消息
		var endMsg []message.MessageSegment
		endMsg = append(endMsg, message.Text(fmt.Sprintf("话题名：%s\n", trimmedName)))
		for i, msg := range mList {
			// 将CQ码中的图片URL替换为本地路径
			msg := replaceImageUrlWithLocalPath(msg, qqNumber)
			if i != 0 { // 两条消息之间添加换行
				endMsg = append(endMsg, message.Text("\n"))
			}
			endMsg = append(endMsg, message.Text(i+1, ". ")) // 给每条笔记添加序号
			endMsg = append(endMsg, msg...)
		}
		ctx.SendChain(endMsg...)
	})

	// 设置话题提醒频率
	engine.OnPrefixGroup([]string{"sztx", "话题提醒"}).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		args := ctx.State["args"].(string)
		fields := strings.Fields(args)
		if len(fields) != 2 {
			logrus.Info("解析失败：参数必须为话题名和提醒频率2个值")
			ctx.SendChain(message.Text("参数必须为话题名和提醒频率2个值"))
			return
		}
		topicName, frequency := fields[0], fields[1]
		topicId, ok := topicMap[ctx.Event.Sender.ID].name2Id[topicName]
		if !ok {
			return
		}
		if !ok {
			logrus.Errorf("话题不存在")
			ctx.SendChain(message.Text("话题不存在"))
			return
		}
		nextRemind, err := remindResolve(frequency)
		if err != nil {
			logrus.Info("提醒规则解析失败：" + err.Error())
			ctx.SendChain(message.Text("提醒规则解析失败：" + err.Error()))
			return
		}

		fmt.Printf("nextRemind.Time=%v", nextRemind.Time)
		// if nextRemind.Time.Sub(time.Now()) < 1*time.Minute && (nextRemind.Time.Before(time.Now()) || nextRemind.Time.Minute() <= time.Now().Minute()) {
		if nextRemind.Time.Before(time.Now()) || (nextRemind.Time.Sub(time.Now()) < 1*time.Minute && nextRemind.Time.Minute() <= time.Now().Minute()) {
			logrus.Info("解析失败：计划提醒时间必须在一分钟之后")
			ctx.SendChain(message.Text("计划提醒时间必须在一分钟之后"))
			return
		}
		qqNumber := strconv.FormatInt(ctx.Event.Sender.ID, 10)

		db := GetDB()
		updateFields := map[string]interface{}{
			"next_remind_time": nextRemind.Time,
			"remind_rule":      nextRemind.RemindRule,
			"is_repeat":        int8(map[bool]int{false: 0, true: 1}[nextRemind.IsRepeat]),
		}
		db.Model(&model.Remind{}).Where("qq_number = ? AND topic_id = ?", qqNumber, topicId).Updates(updateFields)

		ctx.SendChain(message.Text(fmt.Sprintf("好的，%s我会把话题：%s的内容发给你", nextRemind.RemindRule, topicName)))
	})

	// 查看记事
	ckjsPrefix := []string{"ckjs", "查看记事"}
	engine.OnMessage().SetBlock(false).Handle(func(ctx *zero.Ctx) {
		var whichCommand string
		rawMsg := ctx.MessageString()
		for _, prefix := range ckjsPrefix {
			if strings.Contains(rawMsg, prefix) {
				whichCommand = prefix
				break
			}
		}
		if whichCommand == "" {
			return
		}

		re := regexp.MustCompile(`\[CQ:at,qq=(\d+)]`)
		match := re.FindStringSubmatch(rawMsg)
		qqNumber := ctx.Event.Sender.ID
		if len(match) != 0 {
			// 如果查看记事时艾特了别人，则查看这个人的记事内容
			qqNumber, _ = strconv.ParseInt(match[1], 10, 64)
			rawMsg = re.ReplaceAllString(rawMsg, "")
			rawMsg = strings.TrimSpace(rawMsg)
			logrus.Infoln("检测到at:" + match[1])
			logrus.Infoln("删除at后Msg=" + rawMsg)
		}
		params := removePrefix(rawMsg, ckjsPrefix)
		notes, err := queryNotes(strconv.FormatInt(qqNumber, 10), funcTypeJishi, params, -1)
		logrus.Infof("params=%v, notes=%v", params, notes)
		if err != nil {
			logrus.Errorf("查询笔记失败：%v", err)
		}
		var mList []message.Message
		var segList []message.MessageSegment
		idMap := map[int]uint{}
		for i, note := range notes {
			m := message.ParseMessageFromString(note.Content + "\n")
			mList = append(mList, m)
			segList = append(segList, m...)
			idMap[i+1] = note.ID // 记事列表id映射关系
		}
		// 保存该用户记事列表id映射关系
		userInfos[qqNumber] = userInfo{cmdType: funcTypeJishi, idMap: idMap}

		// 把多个单独消息拼接成一条长消息
		var endMsg []message.MessageSegment
		for i, msg := range mList {
			// 将CQ码中的图片URL替换为本地路径
			msg := replaceImageUrlWithLocalPath(msg, qqNumber)
			if i != 0 { // 两条消息之间添加换行
				endMsg = append(endMsg, message.Text("\n"))
			}
			endMsg = append(endMsg, message.Text(i+1, ". ")) // 给每条笔记添加序号
			endMsg = append(endMsg, msg...)
		}
		if len(endMsg) == 0 {
			logrus.Info("记事数量为0")
			ctx.SendChain(message.Text("无结果"))
			return
		}
		ctx.SendChain(endMsg...)
	})

	// 查看定时任务
	cronPrefix := []string{"tx", "查看提醒"}
	engine.OnPrefixGroup(cronPrefix).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		params := removePrefix(ctx.Event.RawMessage, cronPrefix)
		qqNumber := ctx.Event.Sender.ID
		var cronTasks []model.Remind
		db := GetDB()
		if err := db.Debug().Where("qq_number =? and status=?", qqNumber, TaskStatusOn).Find(&cronTasks).Error; err != nil {
			logrus.Errorf("查询笔记失败：%v", err.Error())
			return
		}
		logrus.Infof("params=%v, cronTasks=%v", params, cronTasks)
		var mList []message.Message
		var segList []message.MessageSegment
		idMap := map[int]uint{}
		for i, task := range cronTasks {
			m := message.ParseMessageFromString(task.Content + "\n频率：" + task.RemindRule + "\n下次提醒：" + task.NextRemindTime.Format("2006-01-02 15:04:05") + "\n")
			mList = append(mList, m)
			segList = append(segList, m...)
			idMap[i+1] = task.ID // 记事列表id映射关系
		}
		// 保存该用户记事列表id映射关系
		userInfos[qqNumber] = userInfo{cmdType: funcTypeAutoRemind, idMap: idMap}

		// 把多个单独消息拼接成一条长消息
		var endMsg []message.MessageSegment
		for i, msg := range mList {
			// 将CQ码中的图片URL替换为本地路径
			msg := replaceImageUrlWithLocalPath(msg, qqNumber)
			if i != 0 { // 两条消息之间添加换行
				endMsg = append(endMsg, message.Text("\n"))
			}
			endMsg = append(endMsg, message.Text(i+1, ". ")) // 给每条笔记添加序号
			endMsg = append(endMsg, msg...)
		}
		if len(endMsg) == 0 {
			logrus.Info("无提醒数据")
			ctx.SendChain(message.Text("无提醒数据"))
			return
		}
		ctx.SendChain(endMsg...)
	})

	// 查看全部话题
	engine.OnPrefixGroup([]string{"qbht", "全部话题", "ckht", "查看话题"}).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		qqNumber := ctx.Event.Sender.ID
		var topicTasks []model.Remind
		db := GetDB()
		if err := db.Debug().Where("type =? and qq_number =?", TaskTypeTopic, qqNumber).Find(&topicTasks).Error; err != nil {
			logrus.Errorf("查询话题失败：%v", err.Error())
			return
		}
		var topicIds []int
		for _, task := range topicTasks {
			topicIds = append(topicIds, task.TopicId)
		}

		var result []struct {
			Type  int
			Count int
		}
		db.Model(&model.Note{}).
			Select("type, count(*) as count").
			Where("qq_number = ? and type in ? and is_delete = ?", qqNumber, topicIds, NoDeleted).
			Group("type").
			Scan(&result)
		countMap := make(map[int]int)
		for _, r := range result {
			countMap[r.Type] = r.Count
		}

		fmt.Printf("话题数量统计result=%v， topicTasks=%v, countMap=%v\n", result, topicTasks, countMap)

		var segList []message.MessageSegment
		idMap := map[int]uint{}
		for i, topic := range topicTasks {
			strs := []string{
				fmt.Sprintf("%d. ", i+1) + topic.TopicName,
				"内容数量：" + strconv.Itoa(countMap[topic.TopicId]),
				"提醒频率：" + topic.RemindRule,
				"下次提醒：" + topic.NextRemindTime.Format("2006-01-02 15:04:05"),
				"是否开启提醒：" + map[int8]string{0: "否", 1: "是"}[topic.Status],
				"\n",
			}
			segList = append(segList, message.Text(strings.Join(strs, "\n")))
			idMap[i+1] = topic.ID // 话题列表id映射关系
		}
		// 保存该用户记事列表id映射关系
		userInfos[qqNumber] = userInfo{cmdType: funcTypeTopic, idMap: idMap}
		if len(segList) == 0 {
			logrus.Info("无话题")
			ctx.SendChain(message.Text("无话题"))
			return
		}
		ctx.SendChain(segList...)
	})

	// 删除记事
	engine.OnPrefixGroup([]string{"del"}).SetBlock(false).Handle(func(ctx *zero.Ctx) {
		args := ctx.State["args"].(string)
		fmt.Printf("要删除的序号=%v\n", args)
		userInfo, ok := userInfos[ctx.Event.Sender.ID]
		if !ok {
			ctx.SendChain(message.Text("请先查询出你要删除的内容"))
			return
		}
		fmt.Printf("userInfo=%v\n", userInfo)
		tmpIds := extractIds(args)
		var ids []uint
		for _, tmpId := range tmpIds {
			ids = append(ids, userInfo.idMap[tmpId])
		}
		fmt.Printf("要删除的id=%v\n", ids)
		db := GetDB()

		var whichModel interface{}
		var errMsg string
		var updates map[string]interface{}
		var resultList []map[string]interface{}

		switch userInfo.cmdType {
		case funcTypeJishi, funcTypeTopic:
			whichModel = model.Note{}
			updates = map[string]interface{}{"is_delete": Deleted}
		case funcTypeAutoRemind:
			whichModel = model.Remind{}
			updates = map[string]interface{}{"status": TaskStatusOff}
		default:
			errMsg = fmt.Sprintf("未知命令类型%v", userInfo.cmdType)
			logrus.Errorf(errMsg)
			ctx.SendChain(message.Text(errMsg))
			return
		}

		if err := db.Debug().Model(&whichModel).Where("id in (?)", ids).Scan(&resultList).Error; err != nil {
			errMsg = fmt.Sprintf("查询待删除内容失败：%v", err.Error())
			logrus.Errorf(errMsg)
			ctx.SendChain(message.Text(errMsg))
			return
		}
		logrus.Infof("待删除内容resultList=%v", resultList)
		var mList []message.Message
		var segList []message.MessageSegment
		for _, result := range resultList {
			m := message.ParseMessageFromString(result["content"].(string) + "\n")
			mList = append(mList, m)
			segList = append(segList, m...)
		}

		// 把多个单独消息拼接成一条长消息
		endMsg := []message.MessageSegment{message.Text("是否确认删除？\n")}
		for i, msg := range mList {
			if i != 0 { // 两条消息之间添加换行
				endMsg = append(endMsg, message.Text("\n"))
			}
			endMsg = append(endMsg, message.Text(i+1, ". ")) // 给每条笔记添加序号
			endMsg = append(endMsg, msg...)
		}
		// 发送要删除的内容，询问用户是否确认删除
		ctx.Send(endMsg)
		next := zero.NewFutureEvent("message", 999, true, ctx.CheckSession(), func(ctx *zero.Ctx) bool {
			return php2go.InArray(strings.TrimSpace(ctx.Event.RawMessage), []string{"y", "yes", "是", "确认"})
		}).Next()
		for {
			select {
			case <-time.After(time.Second * 30):
				ctx.SendChain(message.Text("未确认，删除操作取消"))
				logrus.Infoln("未确认，删除操作取消")
				return
			case <-next:
				updateResult := db.Model(&whichModel).Where("id in (?)", ids).Updates(updates)
				if updateResult.Error != nil {
					errMsg = fmt.Sprintf("删除失败：%v", updateResult.Error)
					logrus.Errorf(errMsg)
					ctx.SendChain(message.Text(errMsg))
					return
				}

				ctx.SendChain(message.Text(fmt.Sprintf("应删：%v，实删%v", len(ids), updateResult.RowsAffected)))
				return
			}
		}
	})
}

func replaceImageUrlWithLocalPath(msg message.Message, qqNumber int64) message.Message {
	updatedMsg := make(message.Message, len(msg))
	for i, elem := range msg {
		if elem.Type == "image" {
			localName, ok := elem.Data["local_name"]
			if ok {
				relPath := filepath.Join("data", strconv.FormatInt(qqNumber, 10), "images", localName)
				absPath, err := filepath.Abs(relPath)
				if err != nil {
					logrus.Info(fmt.Errorf("failed to get absolute path: %v. Error: %v", absPath, err))
				} else {
					absPath = filepath.ToSlash(absPath)
					imageContent, err := os.ReadFile(absPath)
					if err != nil {
						logrus.Info(fmt.Errorf("failed to read absolute path: %v. Error: %v", absPath, err))
					} else {
						updatedMsg[i] = message.ImageBytes(imageContent)
						continue
					}
				}
			}
		}
		updatedMsg[i] = elem
	}
	return updatedMsg
}

func downloadImage(ctx *zero.Ctx, url string, senderId int64, maxRetry int) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create http request: %v", err)
	}

	// 设置伪装浏览器header
	var headers = map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3",
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	var resp *http.Response
	for i := 0; i < maxRetry; i++ {
		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		fmt.Printf("Retry %d times due to error: %v\n", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return "", fmt.Errorf("failed to download image after %d retries: %v", maxRetry, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// 获取文件扩展名
	_, imageFormat, err := image.DecodeConfig(bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %v", err)
	}

	// 生成文件名
	filename := time.Now().Format("2006-01-02T150405.000000") + "." + imageFormat

	// 获取相对路径
	relPath := filepath.Join("data", strconv.FormatInt(senderId, 10), "images")
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	// 创建目录
	err = os.MkdirAll(absPath, os.ModePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	// 将图片保存到本地
	filePath := filepath.Join(absPath, filename)
	err = os.WriteFile(filePath, body, 0666)
	if err != nil {
		return "", fmt.Errorf("failed to save image: %v", err)
	}

	fmt.Printf("Image saved as %s\n", filename)

	// 上传到群文件
	// ctx.UploadGroupFile(718853660, filePath, filename, "")
	// files := ctx.GetThisGroupRootFiles(0)
	// fmt.Printf("files=%v", files)
	// upResp := ctx.UploadThisGroupFile(filePath, filename, "/131a5840-3c2f-4fe7-88bc-42029bd2d931")
	// fmt.Printf("上传群文件结果：%v", upResp)

	return filename, nil
}

func queryNotes(userId string, noteType int, keyword string, limit int) ([]model.Note, error) {
	db := GetDB()
	var notes []model.Note
	// queryDb := db.Select("id, content, note_url").Where(model.Note{QQNumber: userId, Type: noteType, IsDelete: 0}).Order("cdate desc")
	queryDb := db.Where(model.Note{QQNumber: userId, Type: noteType, IsDelete: 0}, "qq_number", "type", "is_delete").Order("cdate desc")
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
	// folders := filesOfGroup.Get("folders").Array()
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
	// if len(folders) != 0 {
	//	for _, folderNameOflist := range folders {
	//		folderID := folderNameOflist.Get("folder_id").String()
	//		fileSearchName, fileURL = getFileURLbyfolderID(ctx, fileName, folderID)
	//		if fileSearchName != "" {
	//			return
	//		}
	//	}
	// }
	// return
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

// 提取参数里的ID
func extractIds(str string) []int {
	var ids []int
	str = strings.TrimFunc(str, func(r rune) bool {
		return !unicode.IsDigit(r)
	})
	nums := regexp.MustCompile("[ ,]+").Split(str, -1) // 使用正则表达式来支持逗号和空格分隔符
	for _, num := range nums {
		if strings.Contains(num, "-") { // 处理范围
			rangeNums := strings.Split(num, "-")
			start, _ := strconv.Atoi(rangeNums[0])
			end, _ := strconv.Atoi(rangeNums[1])
			for i := start; i <= end; i++ {
				ids = append(ids, i)
			}
		} else if num != "" { // 处理单个ID
			id, _ := strconv.Atoi(num)
			ids = append(ids, id)
		}
	}
	return ids
}
