package model

import (
	"time"
)

type Remind struct {
	ID             uint      `gorm:"column:id;primaryKey"`
	Type           int8      `gorm:"column:type;type:tinyint;not null;default:0;comment:'提醒类型 1学习提醒 2待办提醒 3话题'"`
	QQNumber       string    `gorm:"column:qq_number;type:varchar(10);not null;default:'0';comment:'QQ号码'"`
	GroupNumber    string    `gorm:"column:group_number;type:varchar(10);not null;default:'0';comment:'群号码'"`
	RemindRule     string    `gorm:"column:remind_rule;type:varchar(50);not null;default:'0';comment:'定时提醒规则'"`
	Content        string    `gorm:"column:content;type:text;comment:'提醒内容'"`
	LastRemindTime time.Time `gorm:"column:last_remind_time;default:NULL;comment:'上次提醒时间'"`
	NextRemindTime time.Time `gorm:"column:next_remind_time;default:NULL;comment:'下次提醒时间'"`
	Status         int8      `gorm:"column:status;type:tinyint;not null;default:0;comment:'提醒启用标志 0停用 1启用'"`
	IsRepeat       int8      `gorm:"column:is_repeat;type:tinyint;not null;default:0;comment:'1重复提醒 0一次性提醒'"`
	CDate          time.Time `gorm:"column:cdate;type:timestamp;not null;default:CURRENT_TIMESTAMP;comment:'笔记添加时间'"`
	TopicId        int       `gorm:"column:topic_id;type:int;not null;default:0;comment:'话题id'"`
	TopicName      string    `gorm:"column:topic_name;type:varchar(255);not null;default:'';comment:'话题名称'"`
}

func (r *Remind) TableName() string {
	return "remind"
}
