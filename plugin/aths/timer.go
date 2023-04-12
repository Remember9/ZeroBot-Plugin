package aths

import (
	"fmt"
	"github.com/FloatTech/ZeroBot-Plugin/plugin/aths/model"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"gorm.io/gorm/clause"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	TaskTypeReview = 1
	TaskTypeTodo   = 2

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
}

type RemindData struct {
	RemindQQ   string
	Time       time.Time
	RemindRule string
	Content    string
	IsRepeat   bool
}

//var remindRules = map[int]string{
//	TypeMinute:                         "^(\\d+)分(?:钟)?后$",
//	TypeHour:                           "^(\\d+)小时后$",
//	TypeTodaySpecifyTime:               "^(\\d+)点(\\d+)?(?:分)?$",
//	TypeTomorrowSpecifyTime:            "^明天(\\d+)点(\\d+)?(?:分)?$",
//	TypeTheDayAfterTomorrowSpecifyTime: "^后天(\\d+)点(\\d+)?(?:分)?$",
//	TypeYmdhms:                         "^(\\d+)月(\\d+)(?:日|号)(\\d+)点(\\d+)?(?:分)?$",
//	TypePerMinute:                      "^每(\\d+)分(?:钟)?$",
//	TypePerHour:                        "^每(\\d+)小时$",
//	TypePerDay:                         "^每天(\\d+)点(\\d+)?(?:分)?$",
//}

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
	if err := remindDB.Debug().Model(&model.Remind{}).Select("id, next_remind_time, content, last_remind_time, remind_rule, qq_number, group_number, is_repeat").
		Where("type = ? AND status = ? AND next_remind_time <= ?", TaskTypeTodo, TaskStatusOn, nowStr).
		Find(&reminds).Error; err != nil {
		logrus.Errorf("[CheckReminderEvents] Failed to get reminder events: %v", err)
		return
	}
	update := make([]map[string]interface{}, 0)
	for _, row := range reminds {
		sendContent := ""
		if row["content"] != nil && row["content"].(string) != "" {
			sendContent = row["content"].(string)
		} else {
			sendContent = "到点了，到点了！(" + row["remind_rule"].(string) + ")"
		}
		qqNumher, _ := strconv.Atoi((row["qq_number"].(string)))
		groupNumber, _ := strconv.Atoi((row["group_number"].(string)))
		// 私发提醒
		ctx.SendPrivateMessage(int64(qqNumher), message.Text(sendContent))
		// 如果是在群里建立的定时提醒，还会在群里at并提醒
		if row["group_number"] != nil && row["group_number"].(string) != "" {
			msg := message.Message{message.At(int64(qqNumher)), message.Text(sendContent)}
			ctx.SendGroupMessage(int64(groupNumber), msg)
		}
		status := TaskStatusOff
		if row["is_repeat"].(int) == 1 {
			status = TaskStatusOn
		}
		updateItem := map[string]interface{}{
			"id":               row["id"].(int),
			"last_remind_time": time.Now().Format("2006-01-02 15:04:05"),
			"status":           status,
			"next_remind_time": row["next_remind_time"].(string),
		}
		if row["is_repeat"].(int) == 1 {
			nextRemind, err := remindResolve(row["remind_rule"].(string))
			if err != nil {
				errMsg := fmt.Sprintf("提醒规则: %s, 错误信息%s", row["remind_rule"].(string), err.Error())
				logrus.Errorf(errMsg)
				ctx.SendChain(message.Text(errMsg))
				return
			}
			updateItem["next_remind_time"] = nextRemind.Time
			logrus.WithFields(logrus.Fields{"提醒规则": row["remind_rule"].(string), "错误信息": err.Error()}).Error("更新下次提醒时间失败")
		}
		update = append(update, updateItem)
	}

	// 获取待办的全部id
	ids := make([]int, len(update))
	for i, m := range update {
		if id, ok := m["id"].(int); ok {
			ids[i] = id
		}
	}
	logStr += fmt.Sprintf("待办个数: %s, ids=%v", strconv.Itoa(len(update)), ids)
	// 根据remind_rule更新next_remind_time
	if len(update) > 0 {
		result := remindDB.Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{"last_remind_time", "status", "next_remind_time"})}).
			Create(&update)
		if result.Error != nil {
			logrus.WithFields(logrus.Fields{"影响行数": 0, "error": result.Error.Error()}).Error("更新提醒任务失败")
		} else {
			logrus.WithFields(logrus.Fields{"影响行数": int(result.RowsAffected), "更新结果": update}).Info("更新提醒任务成功")
		}
	}

	etime := time.Now().Unix()
	logStr += "耗时：" + strconv.FormatInt(etime-stime, 10) + "秒"
	logrus.Info(logStr)
}

func remindResolve(remindMsg string) (RemindData, error) {
	var matchType int
	var remindParams []string

	// 提醒别人
	remindQq := ""
	atIndex := strings.Index(remindMsg, "@")
	if atIndex != -1 {
		remindAt := remindMsg[atIndex:]
		remindQq = strings.Split(remindAt, " ")[0]
		remindMsg = strings.ReplaceAll(remindMsg, remindAt, "")
	}

	// 解析待办命令
	todoRule, todoThing := "", ""
	if strings.Contains(remindMsg, "提醒他") || strings.Contains(remindMsg, "提醒她") || strings.Contains(remindMsg, "提醒它") || strings.Contains(remindMsg, "提醒ta") {
		todoRuleAndThing := strings.SplitN(remindMsg, "提醒他", 2)
		todoRule = strings.Trim(todoRuleAndThing[0], " ")
		todoThing = strings.Trim(todoRuleAndThing[1], " ")
	} else {
		todoRuleAndThing := strings.SplitN(remindMsg, "提醒我", 2)
		todoRule = strings.Trim(todoRuleAndThing[0], " ")
		todoThing = strings.Trim(todoRuleAndThing[1], " ")
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
		nextRemindTime = nextRemindTime.AddDate(0, 0, 1)
	}

	remindData := RemindData{
		RemindQQ:   remindQq,
		Time:       nextRemindTime,
		RemindRule: todoRule,
		Content:    todoThing,
		IsRepeat:   matchType >= TypePerMinute && matchType <= TypePerDay,
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
