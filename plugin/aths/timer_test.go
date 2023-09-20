package aths

import (
	"fmt"
	"testing"
)

func Test_remindResolve(t *testing.T) {
	for i := 0; i <= 7; i++ {
		resolve, err := remindResolve(fmt.Sprintf("每周%d的20点25提醒我保险", i))
		fmt.Printf("%+v, err=%v", resolve, err)
	}
	for k := range weekCnMapping {
		resolve, err := remindResolve(fmt.Sprintf("每周%s的20点25提醒我保险", k))
		fmt.Printf("%+v, err=%v", resolve, err)
	}
}
