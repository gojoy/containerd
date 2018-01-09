package migration

import "testing"

var id string="0a4e9597c1741c1ae755beda85461030ca87aed304292a7993f76ec4fe2a75fe"

func TestGetDir(t *testing.T) {

	lower,err:=GetDir(id)
	if err!=nil {
		t.Errorf("error:%v\n",err)
	}
	for _,v:=range lower {
		println(v)
	}

}
