package aths

import (
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
	"time"
)

func Test_remindResolve(t *testing.T) {
	for i := 1; i <= 7; i++ {
		resolve, err := remindResolve(fmt.Sprintf("每周%d的20点25提醒我保险", i))
		fmt.Printf("%+v, err=%v", resolve, err)
	}
	for k := range weekCnMapping {
		resolve, err := remindResolve(fmt.Sprintf("每周%s的0点25提醒我保险", k))
		fmt.Printf("%+v, err=%v", resolve, err)
	}
}

func Test_remindResolve1(t *testing.T) {
	Convey("解析提醒", t, func() {
		resolve, err := remindResolve("16点10提醒我站起来")
		So(err, ShouldBeNil)
		now := time.Now()
		So(resolve.Time, ShouldEqual, time.Date(now.Year(), now.Month(), now.Day(), 16, 10, 0, 0, now.Location()))
		So(resolve.Content, ShouldEqual, "站起来")
		So(resolve.IsRepeat, ShouldEqual, false)
	})
	Convey("解析提醒", t, func() {
		resolve, err := remindResolve("明天16点10提醒我站起来")
		So(err, ShouldBeNil)
		now := time.Now()
		So(resolve.Time, ShouldEqual, time.Date(now.Year(), now.Month(), now.Day()+1, 16, 10, 0, 0, now.Location()))
		So(resolve.Content, ShouldEqual, "站起来")
		So(resolve.IsRepeat, ShouldEqual, false)
	})
	Convey("解析提醒", t, func() {
		resolve, err := remindResolve("每周五16点10提醒我站起来")
		So(err, ShouldBeNil)
		now := time.Now()
		So(resolve.Time, ShouldEqual, time.Date(now.Year(), now.Month(), 19, 16, 10, 0, 0, now.Location()))
		So(resolve.Content, ShouldEqual, "站起来")
		So(resolve.IsRepeat, ShouldEqual, true)
	})
}
