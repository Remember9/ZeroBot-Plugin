package model

import (
	"time"
)

type Note struct {
	ID             uint      `gorm:"column:id;primaryKey"`
	Type           int8      `gorm:"column:type;type:tinyint;not null;default:0;comment:'笔记类型'"`
	QQNumber       string    `gorm:"column:qq_number;type:varchar(10);not null;default:'0';comment:'QQ号码'"`
	GroupNumber    string    `gorm:"column:group_number;type:varchar(10);not null;default:'0';comment:'群号码'"`
	Content        string    `gorm:"column:content;type:text;not null;comment:'笔记内容'"`
	ImgURLs        string    `gorm:"column:img_urls;type:json;default:NULL;comment:'图片url'"`
	NoteURL        string    `gorm:"column:note_url;type:varchar(200);not null;default:'';comment:'笔记线上url'"`
	BrainURL       string    `gorm:"column:brain_url;type:varchar(200);not null;default:'';comment:'脑图url'"`
	IsReview       int8      `gorm:"column:is_review;type:tinyint;not null;default:1;comment:'1 复习 2 不复习'"`
	ReviewCount    int8      `gorm:"column:review_count;type:tinyint;not null;default:0;comment:'复习次数'"`
	NextReviewTime time.Time `gorm:"column:next_review_time;default:NULL;comment:'下次复习时间'"`
	LastReviewTime time.Time `gorm:"column:last_review_time;default:NULL;comment:'上次复习时间'"`
	CDate          time.Time `gorm:"column:cdate;type:timestamp;not null;default:CURRENT_TIMESTAMP;comment:'笔记添加时间'"`
	UDate          time.Time `gorm:"column:udate;type:timestamp;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;comment:'更新时间'"`
	IsDelete       int8      `gorm:"column:is_delete;type:tinyint;not null;default:0;comment:'删除标志 0未删除 1已删除'"`
}

func (n *Note) TableName() string {
	return "note"
}
