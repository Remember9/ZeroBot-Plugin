package aths

import (
	"fmt"
	"github.com/FloatTech/ZeroBot-Plugin/plugin/aths/model"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	TaskTypeReview = 1
	TaskTypeTodo   = 2
	TaskTypeTopic  = 3

	TaskStatusOn  = 1
	TaskStatusOff = 0

	TypeMinute                         = 1
	TypeHour                           = 2
	TypeTodaySpecifyTime               = 3
	TypeTomorrowSpecifyTime            = 4
	TypeTheDayAfterTomorrowSpecifyTime = 5
	TypeYmdhms                         = 6
	TypePerMinute                      = 7
	TypePerHour                        = 8
	TypePerDay                         = 9
	TypePerWeek                        = 10
)

var remindRules = map[int]*regexp.Regexp{
	TypeMinute:                         regexp.MustCompile(`^(\d+)分(?:钟)?后$`),
	TypeHour:                           regexp.MustCompile(`^(\d+)小时后$`),
	TypeTodaySpecifyTime:               regexp.MustCompile(`^(\d+)点(\d+)?(?:分)?$`),
	TypeTomorrowSpecifyTime:            regexp.MustCompile(`^明天(\d+)点(\d+)?(?:分)?$`),
	TypeTheDayAfterTomorrowSpecifyTime: regexp.MustCompile(`^后天(\d+)点(\d+)?(?:分)?$`),
	TypeYmdhms:                         regexp.MustCompile(`^(\d+)月(\d+)(?:日|号)(\d+)点(\d+)?(?:分)?$`),
	TypePerMinute:                      regexp.MustCompile(`^每(\d+)分(?:钟)?$`),
	TypePerHour:                        regexp.MustCompile(`^每(\d+)小时$`),
	TypePerDay:                         regexp.MustCompile(`^每天(\d+)点(\d+)?(?:分)?$`),
	TypePerWeek:                        regexp.MustCompile(`^每周([1234567一二三四五六七日])的(\d+)点(\d+)?(?:分)?$`),
}

var weekCnMapping = map[string]int{
	"一": 1,
	"二": 2,
	"三": 3,
	"四": 4,
	"五": 5,
	"六": 6,
	"日": 7,
	"七": 7,
}

// var remindRules = map[int]*regexp.Regexp{
//	TypeMinute:                         regexp.MustCompile(`^(\d+)分(?:钟)?后$`),
//	TypeHour:                           regexp.MustCompile(`^(\d+)小时后$`),
//	TypeTodaySpecifyTime:               regexp.MustCompile(`^(?:早上|晚上|中午|下午)(\d+)点(\d+)?(?:分)?$`),
//	TypeTomorrowSpecifyTime:            regexp.MustCompile(`^明天(?:早上|晚上|中午|下午)(\d+)点(\d+)?(?:分)?$`),
//	TypeTheDayAfterTomorrowSpecifyTime: regexp.MustCompile(`^后天(?:早上|晚上|中午|下午)(\d+)点(\d+)?(?:分)?$`),
//	TypeYmdhms:                         regexp.MustCompile(`^(\d+)月(\d+)(?:日|号)(?:早上|晚上|中午|下午)(\d+)点(\d+)?(?:分)?$`),
//	TypePerMinute:                      regexp.MustCompile(`^每(\d+)分(?:钟)?$`),
//	TypePerHour:                        regexp.MustCompile(`^每(\d+)小时$`),
//	TypePerDay:                         regexp.MustCompile(`^每天(?:早上|晚上|中午|下午)(\d+)点(\d+)?(?:分)?$`),
// }

type RemindData struct {
	RemindQQ   string
	Time       time.Time
	RemindRule string
	Content    string
	IsRepeat   bool
}

// var remindRules = map[int]string{
//	TypeMinute:                         "^(\\d+)分(?:钟)?后$",
//	TypeHour:                           "^(\\d+)小时后$",
//	TypeTodaySpecifyTime:               "^(\\d+)点(\\d+)?(?:分)?$",
//	TypeTomorrowSpecifyTime:            "^明天(\\d+)点(\\d+)?(?:分)?$",
//	TypeTheDayAfterTomorrowSpecifyTime: "^后天(\\d+)点(\\d+)?(?:分)?$",
//	TypeYmdhms:                         "^(\\d+)月(\\d+)(?:日|号)(\\d+)点(\\d+)?(?:分)?$",
//	TypePerMinute:                      "^每(\\d+)分(?:钟)?$",
//	TypePerHour:                        "^每(\\d+)小时$",
//	TypePerDay:                         "^每天(\\d+)点(\\d+)?(?:分)?$",
// }

func CheckReminderEvents(ctx *zero.Ctx) {
	stime := time.Now().Unix()
	logStr := "[" + time.Unix(stime, 0).Format("2006-01-02 15:04:05") + "]"

	remindDB := GetDB()
	if remindDB == nil {
		// Handle the error
		panic("Failed to connect to the database")
	}
	var reminds []map[string]interface{}
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	if err := remindDB.Debug().Model(&model.Remind{}).
		Where("type in ? AND status = ? AND next_remind_time <= ?", []int{TaskTypeTodo, TaskTypeTopic}, TaskStatusOn, nowStr).
		Find(&reminds).Error; err != nil {
		logrus.Errorf("[CheckReminderEvents] Failed to get reminder events: %v", err)
		return
	}
	logrus.Infof("全部待提醒内容=%v\n", reminds)
	update := make([]map[string]interface{}, 0)
	for _, row := range reminds {
		qqNumber, _ := strconv.Atoi(row["qq_number"].(string))
		groupNumber, _ := strconv.Atoi(row["group_number"].(string))
		// 如果是话题提醒
		if row["type"].(int8) == TaskTypeTopic {
			db := GetDB()
			var notes []model.Note
			db.Debug().Where("qq_number=? AND type = ? AND is_delete=?", row["qq_number"].(string), row["topic_id"].(int), NoDeleted).Order("cdate desc").Find(&notes)
			// 把notes拼接成一条消息
			var mList []message.Message
			var segList []message.MessageSegment
			idMap := map[int]uint{}
			fmt.Printf("话题notes=%v\n", notes)
			for i, note := range notes {
				m := message.ParseMessageFromString(note.Content + "\n")
				mList = append(mList, m)
				segList = append(segList, m...)
				idMap[i+1] = note.ID // 记事列表id映射关系
			}
			// 把多个单独消息拼接成一条长消息
			var endMsg []message.MessageSegment
			endMsg = append(endMsg, message.Text(fmt.Sprintf("话题名：%s\n", row["topic_name"].(string))))
			for i, msg := range mList {
				// 将CQ码中的图片URL替换为本地路径
				msg := replaceImageUrlWithLocalPath(msg, int64(qqNumber))
				if i != 0 { // 两条消息之间添加换行
					endMsg = append(endMsg, message.Text("\n"))
				}
				endMsg = append(endMsg, message.Text(i+1, ". ")) // 给每条笔记添加序号
				endMsg = append(endMsg, msg...)
			}
			ctx.SendPrivateMessage(int64(qqNumber), endMsg)
		} else if row["type"].(int8) == TaskTypeTodo {
			sendContent := ""
			if row["content"] != nil && row["content"].(string) != "" {
				sendContent = row["content"].(string)
			} else {
				sendContent = "到点了，到点了！(" + row["remind_rule"].(string) + ")"
			}
			// 私发提醒
			logrus.Info("remind groupNumber=", groupNumber)
			logrus.Info(", qqNumber", qqNumber)
			ctx.SendPrivateMessage(int64(qqNumber), message.Text(sendContent))
			// 如果是在群里建立的定时提醒，还会在群里at并提醒
			if row["group_number"] != nil && row["group_number"].(string) != "0" {
				msg := message.Message{message.At(int64(qqNumber)), message.Text(sendContent)}
				ctx.SendGroupMessage(int64(groupNumber), msg)
			}
		} else {
			ctx.SendPrivateMessage(int64(qqNumber), message.Text("未知提醒类型: "+row["type"].(string)))
		}
		// 发送结束后更新下次提醒时间
		status := TaskStatusOff
		if row["is_repeat"].(int8) == 1 {
			status = TaskStatusOn
		}
		fmt.Printf("@@@@@@@@@@@@@@@@@@@@@row=%v", row)
		updateItem := map[string]interface{}{
			"id":               row["id"].(uint),
			"last_remind_time": time.Now().Format("2006-01-02 15:04:05"),
			"status":           status,
			"next_remind_time": row["next_remind_time"],
		}
		if row["is_repeat"].(int8) == 1 {
			nextRemind, err := remindResolve(row["remind_rule"].(string))
			if err != nil {
				errMsg := fmt.Sprintf("提醒规则: %s, 错误信息%s", row["remind_rule"].(string), err.Error())
				ctx.SendChain(message.Text(errMsg))
				logrus.WithFields(logrus.Fields{"提醒规则": row["remind_rule"].(string), "错误信息": errMsg}).Error("更新下次提醒时间失败")
				return
			}
			updateItem["next_remind_time"] = nextRemind.Time
		}

		// 成功发送提醒后，更新提醒任务
		result := remindDB.Debug().Model(&model.Remind{}).Where("id", updateItem["id"]).Updates(&updateItem)
		if result.Error != nil {
			logrus.Errorf("更新提醒任务失败, error=%v", result.Error.Error())
		} else {
			logrus.WithFields(logrus.Fields{"影响行数": int(result.RowsAffected), "更新结果": result}).Info("更新提醒任务成功")
		}

		update = append(update, updateItem)
	}

	// 获取待办的全部id
	ids := make([]uint, len(update))
	for i, m := range update {
		fmt.Printf("获取待办的全部id m=%v", m)
		if id, ok := m["id"].(uint); ok {
			ids[i] = id
		}
	}
	logStr += fmt.Sprintf("待办个数: %s, ids=%v", strconv.Itoa(len(update)), ids)
	// 根据remind_rule更新next_remind_time
	logrus.Infof("@@@@@@@@@@@@@@@@@@@@@update=%v", update)
	logrus.Infof("@@@@@@@@@@@@@@@@@@@@@len update=%v", len(update))

	etime := time.Now().Unix()
	logStr += "耗时：" + strconv.FormatInt(etime-stime, 10) + "秒"
	logrus.Info(logStr)
}

func remindResolve(remindMsg string) (RemindData, error) {
	var matchType int
	var remindParams []string

	// 提醒别人
	remindQq := ""
	re := regexp.MustCompile(`\[CQ:at,qq=(\d+)]`)
	match := re.FindStringSubmatch(remindMsg)
	if len(match) != 0 {
		remindQq = match[1]
		remindMsg = re.ReplaceAllString(remindMsg, "")
		logrus.Infoln("检测到at:" + match[1])
		logrus.Infoln("删除at后remindMsg=" + remindMsg)
	}

	// 解析待办命令
	todoRule, todoThing := "", ""
	if remindQq != "" {
		fmt.Printf("regexp.MustCompile(`(叫|提醒(他|她|它|ta)?)`).Split(remindMsg, 2)=%v\n", regexp.MustCompile(`(叫|提醒)我`).Split(remindMsg, 2))
		parts := regexp.MustCompile(`(叫|提醒(他|她|它|ta)?)`).Split(remindMsg, 2)
		todoRule = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			todoThing = parts[1]
		}
	} else {
		fmt.Printf("regexp.MustComle(`(叫|提醒)我`)pi.Split(remindMsg, 2)=%v\n", regexp.MustCompile(`(叫|提醒)我`).Split(remindMsg, 2))
		parts := regexp.MustCompile(`(叫|提醒)我`).Split(remindMsg, 2)
		todoRule = parts[0]
		if len(parts) > 1 {
			todoThing = parts[1]
		}
	}

	for regxType, rule := range remindRules {
		if matches := rule.FindStringSubmatch(todoRule); len(matches) > 0 {
			matchType = regxType
			remindParams = matches[1:]
			break
		}
	}

	if matchType == 0 {
		return RemindData{}, fmt.Errorf("现有提醒规则都不能匹配该命令")
	}

	nextRemindTime := time.Now()
	isRepeat := false
	switch matchType {
	case TypeMinute, TypePerMinute:
		minute, _ := strconv.Atoi(remindParams[0])
		nextRemindTime = nextRemindTime.Add(time.Duration(minute) * time.Minute)
	case TypeHour, TypePerHour:
		hour, _ := strconv.Atoi(remindParams[0])
		nextRemindTime = nextRemindTime.Add(time.Duration(hour) * time.Hour)
	case TypeTodaySpecifyTime:
		hour, _ := strconv.Atoi(remindParams[0])
		minute, _ := strconv.Atoi(remindParams[1])
		nextRemindTime = time.Date(nextRemindTime.Year(), nextRemindTime.Month(), nextRemindTime.Day(), hour, minute, 0, 0, nextRemindTime.Location())
	case TypeTomorrowSpecifyTime:
		hour, _ := strconv.Atoi(remindParams[0])
		minute, _ := strconv.Atoi(remindParams[1])
		nextRemindTime = time.Date(nextRemindTime.Year(), nextRemindTime.Month(), nextRemindTime.Day()+1, hour, minute, 0, 0, nextRemindTime.Location())
	case TypeTheDayAfterTomorrowSpecifyTime:
		hour, _ := strconv.Atoi(remindParams[0])
		minute, _ := strconv.Atoi(remindParams[1])
		nextRemindTime = time.Date(nextRemindTime.Year(), nextRemindTime.Month(), nextRemindTime.Day()+2, hour, minute, 0, 0, nextRemindTime.Location())
	case TypeYmdhms:
		month, _ := strconv.Atoi(remindParams[0])
		day, _ := strconv.Atoi(remindParams[1])
		hour, _ := strconv.Atoi(remindParams[2])
		minute, _ := strconv.Atoi(remindParams[3])
		nextRemindTime = time.Date(nextRemindTime.Year(), time.Month(month), day, hour, minute, 0, 0, nextRemindTime.Location())
	case TypePerDay:
		hour, _ := strconv.Atoi(remindParams[0])
		minute, _ := strconv.Atoi(remindParams[1])
		if nextRemindTime.Hour() > hour || (nextRemindTime.Hour() == hour && nextRemindTime.Minute() >= minute) {
			nextRemindTime = time.Date(nextRemindTime.Year(), nextRemindTime.Month(), nextRemindTime.Day()+1, hour, minute, 0, 0, nextRemindTime.Location())
		} else {
			nextRemindTime = time.Date(nextRemindTime.Year(), nextRemindTime.Month(), nextRemindTime.Day(), hour, minute, 0, 0, nextRemindTime.Location())
		}
		isRepeat = true
	case TypePerWeek:
		var week int
		if weekNum, ok := weekCnMapping[remindParams[0]]; ok {
			week = weekNum
		} else {
			week, _ = strconv.Atoi(remindParams[0])
		}

		hour, _ := strconv.Atoi(remindParams[1])
		minute, _ := strconv.Atoi(remindParams[2])
		// 获取当前时间
		now := time.Now()

		// 计算今天是周几（0=Sunday，1=Monday，...，6=Saturday）
		dayOfWeek := int(now.Weekday())

		// 计算距离最近的下一个周几还有多少天
		daysUntilNextTuesday := (week - dayOfWeek + 7) % 7

		// 计算最近的周二的日期
		nextRemindTime = now.AddDate(0, 0, daysUntilNextTuesday)
		if nextRemindTime.Day() == now.Day() && (hour < now.Hour() || (hour == now.Hour() && minute <= now.Minute())) {
			nextRemindTime = time.Date(nextRemindTime.Year(), nextRemindTime.Month(), nextRemindTime.Day()+7, hour, minute, 0, 0, nextRemindTime.Location())
		} else {
			nextRemindTime = time.Date(nextRemindTime.Year(), nextRemindTime.Month(), nextRemindTime.Day(), hour, minute, 0, 0, nextRemindTime.Location())
		}
		isRepeat = true
	}

	remindData := RemindData{
		RemindQQ:   remindQq,
		Time:       nextRemindTime,
		RemindRule: todoRule,
		Content:    todoThing,
		IsRepeat:   isRepeat,
	}

	return remindData, nil
}

func main() {
	// 测试提醒功能
	remindMsg := "提醒我5分钟后去吃饭"
	remindData, err := remindResolve(remindMsg)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%+v\n", remindData)

	remindMsg = "提醒我明天下午2点开会"
	remindData, err = remindResolve(remindMsg)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%+v\n", remindData)

	remindMsg = "提醒我每天晚上8点看书"
	remindData, err = remindResolve(remindMsg)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%+v\n", remindData)

	remindMsg = "@小明 提醒他10分钟后回复邮件"
	remindData, err = remindResolve(remindMsg)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%+v\n", remindData)
}
