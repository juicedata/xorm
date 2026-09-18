// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tests

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"

	"github.com/stretchr/testify/assert"
)

func TestInsertOne(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type Test struct {
		Id      int64     `xorm:"autoincr pk"`
		Msg     string    `xorm:"varchar(255)"`
		Created time.Time `xorm:"created"`
	}

	assert.NoError(t, testEngine.Sync(new(Test)))

	data := Test{Msg: "hi"}
	_, err := testEngine.Insert(data)
	assert.NoError(t, err)
}

func TestInsertMulti(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	type TestMulti struct {
		Id   int64  `xorm:"int(11) pk"`
		Name string `xorm:"varchar(255)"`
	}

	assert.NoError(t, testEngine.Sync(new(TestMulti)))

	num, err := insertMultiDatas(1,
		append([]TestMulti{}, TestMulti{1, "test1"}, TestMulti{2, "test2"}, TestMulti{3, "test3"}))
	assert.NoError(t, err)
	assert.EqualValues(t, 3, num)
}

func insertMultiDatas(step int, datas any) (num int64, err error) {
	sliceValue := reflect.Indirect(reflect.ValueOf(datas))
	var iLen int64
	if sliceValue.Kind() != reflect.Slice {
		return 0, errors.New("not silce")
	}
	iLen = int64(sliceValue.Len())
	if iLen == 0 {
		return 0, nil
	}

	session := testEngine.NewSession()
	defer session.Close()

	if err = callbackLooper(datas, step,
		func(innerDatas any) error {
			n, e := session.InsertMulti(innerDatas)
			if e != nil {
				return e
			}
			num += n
			return nil
		}); err != nil {
		return 0, err
	} else if num != iLen {
		return 0, fmt.Errorf("num error: %d - %d", num, iLen)
	}
	return num, nil
}

func callbackLooper(datas any, step int, actionFunc func(any) error) (err error) {
	sliceValue := reflect.Indirect(reflect.ValueOf(datas))
	if sliceValue.Kind() != reflect.Slice {
		return errors.New("not slice")
	}
	if sliceValue.Len() <= 0 {
		return nil
	}

	var tempLen int
	processedLen := sliceValue.Len()
	for i := 0; i < sliceValue.Len(); i += step {
		if processedLen > step {
			tempLen = i + step
		} else {
			tempLen = sliceValue.Len()
		}
		var tempInterface []any
		for j := i; j < tempLen; j++ {
			tempInterface = append(tempInterface, sliceValue.Index(j).Interface())
		}
		if err = actionFunc(tempInterface); err != nil {
			return err
		}
		processedLen -= step
	}
	return nil
}

func TestInsertOneIfPkIsPoint(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type TestPoint struct {
		Id      *int64     `xorm:"autoincr pk notnull 'id'"`
		Msg     *string    `xorm:"varchar(255)"`
		Created *time.Time `xorm:"created"`
	}

	assert.NoError(t, testEngine.Sync(new(TestPoint)))
	msg := "hi"
	data := TestPoint{Msg: &msg}
	_, err := testEngine.Insert(&data)
	assert.NoError(t, err)
}

func TestInsertOneIfPkIsPointRename(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	type ID *int64
	type TestPoint2 struct {
		Id      ID         `xorm:"autoincr pk notnull 'id'"`
		Msg     *string    `xorm:"varchar(255)"`
		Created *time.Time `xorm:"created"`
	}

	assert.NoError(t, testEngine.Sync(new(TestPoint2)))
	msg := "hi"
	data := TestPoint2{Msg: &msg}
	_, err := testEngine.Insert(&data)
	assert.NoError(t, err)
}

func TestInsert(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	assertSync(t, new(Userinfo))

	user := Userinfo{
		0, "xiaolunwen", "dev", "lunny", time.Now(),
		Userdetail{Id: 1},
		1.78,
		[]byte{1, 2, 3},
		true,
	}
	cnt, err := testEngine.Insert(&user)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt, "insert not returned 1")
	assert.True(t, user.Uid > 0, "not return id error")

	user.Uid = 0
	cnt, err = testEngine.Insert(&user)
	// Username is unique, so this should return error
	assert.Error(t, err, "insert should fail but no error returned")
	assert.EqualValues(t, 0, cnt, "insert not returned 1")
}

func TestInsertErrorAfterClosureCleanup(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type InsertAfterClosureA struct {
		Id        int64 `xorm:"pk autoincr"`
		Amount    int64
		UpdatedAt int64 `xorm:"updated"`
	}

	type InsertAfterClosureB struct {
		Id        int64  `xorm:"pk autoincr"`
		UpdatedAt int64  `xorm:"updated"`
		Code      string `xorm:"unique"`
	}

	assert.NoError(t, testEngine.Sync(new(InsertAfterClosureA), new(InsertAfterClosureB)))

	seed := &InsertAfterClosureA{Amount: 42}
	_, err := testEngine.Insert(seed)
	assert.NoError(t, err)

	session := testEngine.NewSession()
	defer session.Close()

	_, err = session.Insert(&InsertAfterClosureB{Code: "dup"})
	assert.NoError(t, err)

	_, err = session.Insert(&InsertAfterClosureB{Code: "dup"})
	assert.Error(t, err)

	var loaded InsertAfterClosureA
	has, err := session.ID(seed.Id).Get(&loaded)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, seed.Amount, loaded.Amount)
}

func TestInsertAutoIncr(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	assertSync(t, new(Userinfo))

	// auto increment insert
	user := Userinfo{
		Username: "xiaolunwen2", Departname: "dev", Alias: "lunny", Created: time.Now(),
		Detail: Userdetail{Id: 1}, Height: 1.78, Avatar: []byte{1, 2, 3}, IsMan: true,
	}
	cnt, err := testEngine.Insert(&user)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)
	assert.Greater(t, user.Uid, int64(0))
}

func TestInsertDefault(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type DefaultInsert struct {
		Id      int64
		Status  int `xorm:"default -1"`
		Name    string
		Created time.Time `xorm:"created"`
		Updated time.Time `xorm:"updated"`
	}

	di := new(DefaultInsert)
	err := testEngine.Sync(di)
	assert.NoError(t, err)

	di2 := DefaultInsert{Name: "test"}
	_, err = testEngine.Omit(testEngine.GetColumnMapper().Obj2Table("Status")).Insert(&di2)
	assert.NoError(t, err)

	has, err := testEngine.Desc("id").Get(di)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, -1, di.Status)
	assert.EqualValues(t, di2.Updated.Unix(), di.Updated.Unix(), di.Updated)
	assert.EqualValues(t, di2.Created.Unix(), di.Created.Unix(), di.Created)
}

func TestInsertDefault2(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type DefaultInsert2 struct {
		Id        int64
		Name      string
		Url       string    `xorm:"text"`
		CheckTime time.Time `xorm:"not null default '2000-01-01 00:00:00'"`
	}

	di := new(DefaultInsert2)
	err := testEngine.Sync(di)
	assert.NoError(t, err)

	di2 := DefaultInsert2{Name: "test"}
	_, err = testEngine.Omit(testEngine.GetColumnMapper().Obj2Table("CheckTime")).Insert(&di2)
	assert.NoError(t, err)

	has, err := testEngine.Desc("id").Get(di)
	assert.NoError(t, err)
	assert.True(t, has)

	has, err = testEngine.NoAutoCondition().Desc("id").Get(&di2)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, *di, di2)
}

type CreatedInsert struct {
	Id      int64
	Created time.Time `xorm:"created"`
}

type CreatedInsert2 struct {
	Id      int64
	Created int64 `xorm:"created"`
}

type CreatedInsert3 struct {
	Id      int64
	Created int `xorm:"created bigint"`
}

type CreatedInsert4 struct {
	Id      int64
	Created int `xorm:"created"`
}

type CreatedInsert5 struct {
	Id      int64
	Created time.Time `xorm:"created bigint"`
}

type CreatedInsert6 struct {
	Id      int64
	Created time.Time `xorm:"created bigint"`
}

func TestInsertCreated(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	di := new(CreatedInsert)
	err := testEngine.Sync(di)
	assert.NoError(t, err)

	ci := &CreatedInsert{}
	_, err = testEngine.Insert(ci)
	assert.NoError(t, err)

	has, err := testEngine.Desc("id").Get(di)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, ci.Created.Unix(), di.Created.Unix())

	di2 := new(CreatedInsert2)
	err = testEngine.Sync(di2)
	assert.NoError(t, err)

	ci2 := &CreatedInsert2{}
	_, err = testEngine.Insert(ci2)
	assert.NoError(t, err)

	has, err = testEngine.Desc("id").Get(di2)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, ci2.Created, di2.Created)

	di3 := new(CreatedInsert3)
	err = testEngine.Sync(di3)
	assert.NoError(t, err)

	ci3 := &CreatedInsert3{}
	_, err = testEngine.Insert(ci3)
	assert.NoError(t, err)

	has, err = testEngine.Desc("id").Get(di3)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, ci3.Created, di3.Created)

	di4 := new(CreatedInsert4)
	err = testEngine.Sync(di4)
	assert.NoError(t, err)

	ci4 := &CreatedInsert4{}
	_, err = testEngine.Insert(ci4)
	assert.NoError(t, err)

	has, err = testEngine.Desc("id").Get(di4)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, ci4.Created, di4.Created)

	di5 := new(CreatedInsert5)
	err = testEngine.Sync(di5)
	assert.NoError(t, err)

	ci5 := &CreatedInsert5{}
	_, err = testEngine.Insert(ci5)
	assert.NoError(t, err)

	has, err = testEngine.Desc("id").Get(di5)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, ci5.Created.Unix(), di5.Created.Unix())

	di6 := new(CreatedInsert6)
	err = testEngine.Sync(di6)
	assert.NoError(t, err)

	oldTime := time.Now().Add(-time.Hour)
	ci6 := &CreatedInsert6{Created: oldTime}
	_, err = testEngine.Insert(ci6)
	assert.NoError(t, err)

	has, err = testEngine.Desc("id").Get(di6)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, ci6.Created.Unix(), di6.Created.Unix())
}

func TestInsertTime(t *testing.T) {
	type InsertTimeStruct struct {
		Id        int64
		CreatedAt time.Time `xorm:"created"`
		UpdatedAt time.Time `xorm:"updated"`
		DeletedAt time.Time `xorm:"deleted"`
		Stime     time.Time
		Etime     time.Time
	}

	assert.NoError(t, PrepareEngine())
	assertSync(t, new(InsertTimeStruct))

	its := &InsertTimeStruct{
		Stime: time.Now(),
		Etime: time.Now(),
	}
	cnt, err := testEngine.Insert(its)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	var itsGet InsertTimeStruct
	has, err := testEngine.ID(1).Get(&itsGet)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.False(t, itsGet.Stime.IsZero())
	assert.False(t, itsGet.Etime.IsZero())

	var itsFind []*InsertTimeStruct
	err = testEngine.Find(&itsFind)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, len(itsFind))
	assert.False(t, itsFind[0].Stime.IsZero())
	assert.False(t, itsFind[0].Etime.IsZero())
}

type JSONTime time.Time

func (j JSONTime) format() string {
	t := time.Time(j)
	if t.IsZero() {
		return ""
	}

	return t.Format("2006-01-02")
}

func (j JSONTime) MarshalText() ([]byte, error) {
	return []byte(j.format()), nil
}

func (j JSONTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + j.format() + `"`), nil
}

func TestDefaultTime3(t *testing.T) {
	type PrepareTask struct {
		Id int `xorm:"not null pk autoincr INT(11)" json:"id"`
		// ...
		StartTime JSONTime `xorm:"not null default '2006-01-02 15:04:05' TIMESTAMP index" json:"start_time"`
		EndTime   JSONTime `xorm:"not null default '2006-01-02 15:04:05' TIMESTAMP" json:"end_time"`
		Cuser     string   `xorm:"not null default '' VARCHAR(64) index" json:"cuser"`
		Muser     string   `xorm:"not null default '' VARCHAR(64)" json:"muser"`
		Ctime     JSONTime `xorm:"not null default CURRENT_TIMESTAMP TIMESTAMP created" json:"ctime"`
		Mtime     JSONTime `xorm:"not null default CURRENT_TIMESTAMP TIMESTAMP updated" json:"mtime"`
	}

	assert.NoError(t, PrepareEngine())
	assertSync(t, new(PrepareTask))

	prepareTask := &PrepareTask{
		StartTime: JSONTime(time.Now()),
		Cuser:     "userId",
		Muser:     "userId",
	}
	cnt, err := testEngine.Omit("end_time").Insert(prepareTask)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)
}

type MyJSONTime struct {
	Id      int64    `json:"id"`
	Created JSONTime `xorm:"created" json:"created_at"`
}

func TestCreatedJsonTime(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	di5 := new(MyJSONTime)
	err := testEngine.Sync(di5)
	assert.NoError(t, err)

	ci5 := &MyJSONTime{}
	_, err = testEngine.Insert(ci5)
	assert.NoError(t, err)

	has, err := testEngine.Desc("id").Get(di5)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, time.Time(ci5.Created).Unix(), time.Time(di5.Created).Unix())

	dis := make([]MyJSONTime, 0)
	err = testEngine.Find(&dis)
	assert.NoError(t, err)
}

func TestInsertMulti2(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	assertSync(t, new(Userinfo))

	users := []Userinfo{
		{Username: "xlw", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "xlw2", Departname: "dev", Alias: "lunny3", Created: time.Now()},
		{Username: "xlw11", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "xlw22", Departname: "dev", Alias: "lunny3", Created: time.Now()},
	}
	cnt, err := testEngine.Insert(&users)
	assert.NoError(t, err)
	assert.EqualValues(t, len(users), cnt)

	users2 := []*Userinfo{
		{Username: "1xlw", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "1xlw2", Departname: "dev", Alias: "lunny3", Created: time.Now()},
		{Username: "1xlw11", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "1xlw22", Departname: "dev", Alias: "lunny3", Created: time.Now()},
	}

	cnt, err = testEngine.Insert(&users2)
	assert.NoError(t, err)
	assert.EqualValues(t, len(users2), cnt)
}

func TestInsertMulti2Interface(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	assertSync(t, new(Userinfo))

	users := []any{
		Userinfo{Username: "xlw", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		Userinfo{Username: "xlw2", Departname: "dev", Alias: "lunny3", Created: time.Now()},
		Userinfo{Username: "xlw11", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		Userinfo{Username: "xlw22", Departname: "dev", Alias: "lunny3", Created: time.Now()},
	}

	cnt, err := testEngine.Insert(&users)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	assert.EqualValues(t, len(users), cnt)

	users2 := []any{
		&Userinfo{Username: "1xlw", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		&Userinfo{Username: "1xlw2", Departname: "dev", Alias: "lunny3", Created: time.Now()},
		&Userinfo{Username: "1xlw11", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		&Userinfo{Username: "1xlw22", Departname: "dev", Alias: "lunny3", Created: time.Now()},
	}

	cnt, err = testEngine.Insert(&users2)
	assert.NoError(t, err)
	assert.EqualValues(t, len(users2), cnt)
}

func TestInsertTwoTable(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	assertSync(t, new(Userinfo), new(Userdetail))

	userdetail := Userdetail{ /*Id: 1, */ Intro: "I'm a very beautiful women.", Profile: "sfsaf"}
	userinfo := Userinfo{Username: "xlw3", Departname: "dev", Alias: "lunny4", Created: time.Now(), Detail: userdetail}

	cnt, err := testEngine.Insert(&userinfo, &userdetail)
	assert.NoError(t, err)
	assert.Greater(t, userinfo.Uid, int64(0))
	assert.Greater(t, userdetail.Id, int64(0))
	assert.EqualValues(t, 2, cnt)
}

func TestInsertCreatedInt64(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type TestCreatedInt64 struct {
		Id      int64  `xorm:"autoincr pk"`
		Msg     string `xorm:"varchar(255)"`
		Created int64  `xorm:"created"`
	}

	assert.NoError(t, testEngine.Sync(new(TestCreatedInt64)))

	data := TestCreatedInt64{Msg: "hi"}
	now := time.Now()
	cnt, err := testEngine.Insert(&data)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)
	assert.True(t, now.Unix() <= data.Created)

	var data2 TestCreatedInt64
	has, err := testEngine.Get(&data2)
	assert.NoError(t, err)
	assert.True(t, has)

	assert.EqualValues(t, data.Created, data2.Created)
}

type MyUserinfo Userinfo

func (MyUserinfo) TableName() string {
	return "user_info"
}

func TestInsertMulti3(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	testEngine.ShowSQL(true)
	assertSync(t, new(MyUserinfo))

	users := []MyUserinfo{
		{Username: "xlw", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "xlw2", Departname: "dev", Alias: "lunny3", Created: time.Now()},
		{Username: "xlw11", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "xlw22", Departname: "dev", Alias: "lunny3", Created: time.Now()},
	}
	cnt, err := testEngine.Insert(&users)
	assert.NoError(t, err)
	assert.EqualValues(t, len(users), cnt)

	users2 := []*MyUserinfo{
		{Username: "1xlw", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "1xlw2", Departname: "dev", Alias: "lunny3", Created: time.Now()},
		{Username: "1xlw11", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "1xlw22", Departname: "dev", Alias: "lunny3", Created: time.Now()},
	}

	cnt, err = testEngine.Insert(&users2)
	assert.NoError(t, err)
	assert.EqualValues(t, len(users), cnt)
}

type MyUserinfo2 struct {
	Uid        int64  `xorm:"id pk not null autoincr"`
	Username   string `xorm:"unique"`
	Departname string
	Alias      string `xorm:"-"`
	Created    time.Time
	Detail     Userdetail `xorm:"detail_id int(11)"`
	Height     float64
	Avatar     []byte
	IsMan      bool
}

func (MyUserinfo2) TableName() string {
	return "user_info"
}

func TestInsertMulti4(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	testEngine.ShowSQL(false)
	assertSync(t, new(MyUserinfo2))
	testEngine.ShowSQL(true)

	users := []MyUserinfo2{
		{Username: "xlw", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "xlw2", Departname: "dev", Alias: "lunny3", Created: time.Now()},
		{Username: "xlw11", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "xlw22", Departname: "dev", Alias: "lunny3", Created: time.Now()},
	}
	cnt, err := testEngine.Insert(&users)
	assert.NoError(t, err)
	assert.EqualValues(t, len(users), cnt)

	users2 := []*MyUserinfo2{
		{Username: "1xlw", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "1xlw2", Departname: "dev", Alias: "lunny3", Created: time.Now()},
		{Username: "1xlw11", Departname: "dev", Alias: "lunny2", Created: time.Now()},
		{Username: "1xlw22", Departname: "dev", Alias: "lunny3", Created: time.Now()},
	}

	cnt, err = testEngine.Insert(&users2)
	assert.NoError(t, err)
	assert.EqualValues(t, len(users), cnt)
}

func TestAnonymousStruct(t *testing.T) {
	type PlainObject struct {
		ID   uint64 `json:"id,string" xorm:"'ID' pk autoincr"`
		Desc string `json:"desc" xorm:"'DESC' notnull"`
	}

	type PlainFoo struct {
		PlainObject `xorm:"extends"` // primary key defined in extends struct

		Width  uint32 `json:"width" xorm:"'WIDTH' notnull"`
		Height uint32 `json:"height" xorm:"'HEIGHT' notnull"`

		Ext struct {
			F1 uint32 `json:"f1,omitempty"`
			F2 uint32 `json:"f2,omitempty"`
		} `json:"ext" xorm:"'EXT' json notnull"`
	}

	assert.NoError(t, PrepareEngine())
	assertSync(t, new(PlainFoo))

	_, err := testEngine.Insert(&PlainFoo{
		PlainObject: PlainObject{
			Desc: "test",
		},
		Width:  10,
		Height: 20,

		Ext: struct {
			F1 uint32 `json:"f1,omitempty"`
			F2 uint32 `json:"f2,omitempty"`
		}{
			F1: 11,
			F2: 12,
		},
	})
	assert.NoError(t, err)
}

func TestInsertMap(t *testing.T) {
	if testEngine.Dialect().URI().DBType == schemas.DAMENG {
		t.SkipNow()
		return
	}

	type InsertMap struct {
		Id     int64
		Width  uint32
		Height uint32
		Name   string
	}

	assert.NoError(t, PrepareEngine())
	assertSync(t, new(InsertMap))

	cnt, err := testEngine.Table(new(InsertMap)).Insert(map[string]any{
		"width":  20,
		"height": 10,
		"name":   "lunny",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	var im InsertMap
	has, err := testEngine.Get(&im)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, 20, im.Width)
	assert.EqualValues(t, 10, im.Height)
	assert.EqualValues(t, "lunny", im.Name)

	cnt, err = testEngine.Table("insert_map").Insert(map[string]any{
		"width":  30,
		"height": 10,
		"name":   "lunny",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	var ims []InsertMap
	err = testEngine.Find(&ims)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(ims))
	assert.EqualValues(t, 20, ims[0].Width)
	assert.EqualValues(t, 10, ims[0].Height)
	assert.EqualValues(t, "lunny", ims[0].Name)
	assert.EqualValues(t, 30, ims[1].Width)
	assert.EqualValues(t, 10, ims[1].Height)
	assert.EqualValues(t, "lunny", ims[1].Name)

	cnt, err = testEngine.Table("insert_map").Insert([]map[string]any{
		{
			"width":  40,
			"height": 10,
			"name":   "lunny",
		},
		{
			"width":  50,
			"height": 10,
			"name":   "lunny",
		},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 2, cnt)

	ims = make([]InsertMap, 0, 4)
	err = testEngine.Find(&ims)
	assert.NoError(t, err)
	assert.EqualValues(t, 4, len(ims))
	assert.EqualValues(t, 20, ims[0].Width)
	assert.EqualValues(t, 10, ims[0].Height)
	assert.EqualValues(t, "lunny", ims[1].Name)
	assert.EqualValues(t, 30, ims[1].Width)
	assert.EqualValues(t, 10, ims[1].Height)
	assert.EqualValues(t, "lunny", ims[1].Name)
	assert.EqualValues(t, 40, ims[2].Width)
	assert.EqualValues(t, 10, ims[2].Height)
	assert.EqualValues(t, "lunny", ims[2].Name)
	assert.EqualValues(t, 50, ims[3].Width)
	assert.EqualValues(t, 10, ims[3].Height)
	assert.EqualValues(t, "lunny", ims[3].Name)
}

/*
INSERT INTO `issue` (`repo_id`, `poster_id`, ... ,`name`, `content`, ... ,`index`)
SELECT $1, $2, ..., $14, $15, ..., MAX(`index`) + 1 FROM `issue` WHERE `repo_id` = $1;
*/
func TestInsertWhere(t *testing.T) {
	type InsertWhere struct {
		Id     int64
		Index  int   `xorm:"unique(s) notnull"`
		RepoId int64 `xorm:"unique(s)"`
		Width  uint32
		Height uint32
		Name   string
		IsTrue bool
	}

	assert.NoError(t, PrepareEngine())
	assertSync(t, new(InsertWhere))

	i := InsertWhere{
		RepoId: 1,
		Width:  10,
		Height: 20,
		Name:   "trest",
	}

	inserted, err := testEngine.SetExpr("`index`", "coalesce(MAX(`index`),0)+1").
		Where("`repo_id`=?", 1).
		Insert(&i)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, inserted)
	assert.EqualValues(t, 1, i.Id)

	var j InsertWhere
	has, err := testEngine.ID(i.Id).Get(&j)
	assert.NoError(t, err)
	assert.True(t, has)
	i.Index = 1
	assert.EqualValues(t, i, j)

	if testEngine.Dialect().URI().DBType == schemas.DAMENG {
		t.SkipNow()
		return
	}

	inserted, err = testEngine.Table(new(InsertWhere)).Where("`repo_id`=?", 1).
		SetExpr("`index`", "coalesce(MAX(`index`),0)+1").
		Insert(map[string]any{
			"repo_id": 1,
			"width":   20,
			"height":  40,
			"name":    "trest2",
		})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, inserted)

	var j2 InsertWhere
	has, err = testEngine.ID(2).Get(&j2)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, 1, j2.RepoId)
	assert.EqualValues(t, 20, j2.Width)
	assert.EqualValues(t, 40, j2.Height)
	assert.EqualValues(t, "trest2", j2.Name)
	assert.EqualValues(t, 2, j2.Index)

	inserted, err = testEngine.Table(new(InsertWhere)).Where("`repo_id`=?", 1).
		SetExpr("`index`", "coalesce(MAX(`index`),0)+1").
		SetExpr("repo_id", "1").
		Insert(map[string]string{
			"name": "trest3",
		})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, inserted)

	var j3 InsertWhere
	has, err = testEngine.ID(3).Get(&j3)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, "trest3", j3.Name)
	assert.EqualValues(t, 3, j3.Index)

	inserted, err = testEngine.Table(new(InsertWhere)).Where("`repo_id`=?", 1).
		SetExpr("`index`", "coalesce(MAX(`index`),0)+1").
		Insert(map[string]any{
			"repo_id": 1,
			"name":    "10';delete * from insert_where; --",
		})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, inserted)

	var j4 InsertWhere
	has, err = testEngine.ID(4).Get(&j4)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, "10';delete * from insert_where; --", j4.Name)
	assert.EqualValues(t, 4, j4.Index)

	inserted, err = testEngine.Table(new(InsertWhere)).Where("`repo_id`=?", 1).
		SetExpr("`index`", "coalesce(MAX(`index`),0)+1").
		Insert(map[string]any{
			"repo_id": 1,
			"name":    "10\\';delete * from insert_where; --",
		})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, inserted)

	var j5 InsertWhere
	has, err = testEngine.ID(5).Get(&j5)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, "10\\';delete * from insert_where; --", j5.Name)
	assert.EqualValues(t, 5, j5.Index)
}

func TestInsertExpr2(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type InsertExprsRelease struct {
		Id         int64
		RepoId     int
		IsTag      bool
		IsDraft    bool
		NumCommits int
		Sha1       string
	}

	assertSync(t, new(InsertExprsRelease))

	ie := InsertExprsRelease{
		RepoId: 1,
		IsTag:  true,
	}
	inserted, err := testEngine.
		SetExpr("is_draft", true).
		SetExpr("num_commits", 0).
		SetExpr("sha1", "").
		Insert(&ie)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, inserted)

	var ie2 InsertExprsRelease
	has, err := testEngine.ID(ie.Id).Get(&ie2)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, true, ie2.IsDraft)
	assert.EqualValues(t, "", ie2.Sha1)
	assert.EqualValues(t, 0, ie2.NumCommits)
	assert.EqualValues(t, 1, ie2.RepoId)
	assert.EqualValues(t, true, ie2.IsTag)

	if testEngine.Dialect().URI().DBType == schemas.DAMENG {
		t.SkipNow()
		return
	}

	inserted, err = testEngine.Table(new(InsertExprsRelease)).
		SetExpr("is_draft", true).
		SetExpr("num_commits", 0).
		SetExpr("sha1", "").
		Insert(map[string]any{
			"repo_id": 1,
			"is_tag":  true,
		})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, inserted)

	var ie3 InsertExprsRelease
	has, err = testEngine.ID(ie.Id + 1).Get(&ie3)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, true, ie3.IsDraft)
	assert.EqualValues(t, "", ie3.Sha1)
	assert.EqualValues(t, 0, ie3.NumCommits)
	assert.EqualValues(t, 1, ie3.RepoId)
	assert.EqualValues(t, true, ie3.IsTag)
}

type NightlyRate struct {
	ID int64 `xorm:"'id' not null pk BIGINT(20)" json:"id"`
}

func (NightlyRate) TableName() string {
	return "prd_nightly_rate"
}

func TestMultipleInsertTableName(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	tableName := `prd_nightly_rate_16`
	assert.NoError(t, testEngine.Table(tableName).Sync(new(NightlyRate)))

	trans := testEngine.NewSession()
	defer func(trans *xorm.Session) { _ = trans.Close() }(trans)
	err := trans.Begin()
	assert.NoError(t, err)

	rtArr := []any{
		[]*NightlyRate{
			{ID: 1},
			{ID: 2},
		},
		[]*NightlyRate{
			{ID: 3},
			{ID: 4},
		},
		[]*NightlyRate{
			{ID: 5},
		},
	}

	_, err = trans.Table(tableName).Insert(rtArr...)
	assert.NoError(t, err)

	assert.NoError(t, trans.Commit())
}

func TestInsertMultiWithOmit(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type TestMultiOmit struct {
		Id      int64  `xorm:"int(11) pk"`
		Name    string `xorm:"varchar(255)"`
		Omitted string `xorm:"varchar(255) 'omitted'"`
	}

	assert.NoError(t, testEngine.Sync(new(TestMultiOmit)))

	l := []any{
		TestMultiOmit{Id: 1, Name: "1", Omitted: "1"},
		TestMultiOmit{Id: 2, Name: "1", Omitted: "2"},
		TestMultiOmit{Id: 3, Name: "1", Omitted: "3"},
	}

	check := func() {
		var ls []TestMultiOmit
		err := testEngine.Find(&ls)
		assert.NoError(t, err)
		assert.EqualValues(t, 3, len(ls))

		for e := range ls {
			assert.EqualValues(t, "", ls[e].Omitted)
		}
	}

	num, err := testEngine.Omit("omitted").Insert(l...)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, num)
	check()

	num, err = testEngine.Delete(TestMultiOmit{Name: "1"})
	assert.NoError(t, err)
	assert.EqualValues(t, 3, num)

	num, err = testEngine.Omit("omitted").Insert(l)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, num)
	check()
}

func TestInsertTwice(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type InsertStructA struct {
		FieldA int
	}

	type InsertStructB struct {
		FieldB int
	}

	assert.NoError(t, testEngine.Sync(new(InsertStructA), new(InsertStructB)))

	var sliceA []InsertStructA // sliceA is empty
	sliceB := []InsertStructB{
		{
			FieldB: 1,
		},
	}

	ssn := testEngine.NewSession()
	defer func(ssn *xorm.Session) { _ = ssn.Close() }(ssn)

	err := ssn.Begin()
	assert.NoError(t, err)

	_, err = ssn.Insert(sliceA)
	assert.EqualValues(t, xorm.ErrNoElementsOnSlice, err)

	_, err = ssn.Insert(sliceB)
	assert.NoError(t, err)

	assert.NoError(t, ssn.Commit())
}

func TestInsertIntSlice(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type InsertIntSlice struct {
		NameIDs []int `xorm:"json notnull"`
	}

	assert.NoError(t, testEngine.Sync(new(InsertIntSlice)))

	v := InsertIntSlice{
		NameIDs: []int{1, 2},
	}
	cnt, err := testEngine.Insert(&v)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	var v2 InsertIntSlice
	has, err := testEngine.Get(&v2)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, v, v2)

	cnt, err = testEngine.Where("1=1").Delete(new(InsertIntSlice))
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	v3 := InsertIntSlice{
		NameIDs: nil,
	}
	cnt, err = testEngine.Insert(&v3)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	var v4 InsertIntSlice
	has, err = testEngine.Get(&v4)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, v3, v4)
}

func TestInsertDeleted(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type InsertDeletedStructNotRight struct {
		ID        uint64    `xorm:"'ID' pk autoincr"`
		DeletedAt time.Time `xorm:"'DELETED_AT' deleted notnull"`
	}
	// notnull tag will be ignored
	err := testEngine.Sync(new(InsertDeletedStructNotRight))
	assert.NoError(t, err)

	type InsertDeletedStruct struct {
		ID        uint64    `xorm:"'ID' pk autoincr"`
		DeletedAt time.Time `xorm:"'DELETED_AT' deleted"`
	}

	assert.NoError(t, testEngine.Sync(new(InsertDeletedStruct)))

	var v InsertDeletedStruct
	_, err = testEngine.Insert(&v)
	assert.NoError(t, err)

	var v2 InsertDeletedStruct
	has, err := testEngine.Get(&v2)
	assert.NoError(t, err)
	assert.True(t, has)

	_, err = testEngine.ID(v.ID).Delete(new(InsertDeletedStruct))
	assert.NoError(t, err)

	var v3 InsertDeletedStruct
	has, err = testEngine.Get(&v3)
	assert.NoError(t, err)
	assert.False(t, has)

	var v4 InsertDeletedStruct
	has, err = testEngine.Unscoped().Get(&v4)
	assert.NoError(t, err)
	assert.True(t, has)
}

func TestInsertMultipleMap(t *testing.T) {
	if testEngine.Dialect().URI().DBType == schemas.DAMENG {
		t.SkipNow()
		return
	}

	type InsertMultipleMap struct {
		Id     int64
		Width  uint32
		Height uint32
		Name   string
	}

	assert.NoError(t, PrepareEngine())
	assertSync(t, new(InsertMultipleMap))

	cnt, err := testEngine.Table(new(InsertMultipleMap)).Insert([]map[string]any{
		{
			"width":  20,
			"height": 10,
			"name":   "lunny",
		},
		{
			"width":  30,
			"height": 20,
			"name":   "xiaolunwen",
		},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 2, cnt)

	var res []InsertMultipleMap
	err = testEngine.Find(&res)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(res))
	assert.EqualValues(t, InsertMultipleMap{
		Id:     1,
		Width:  20,
		Height: 10,
		Name:   "lunny",
	}, res[0])
	assert.EqualValues(t, InsertMultipleMap{
		Id:     2,
		Width:  30,
		Height: 20,
		Name:   "xiaolunwen",
	}, res[1])

	assert.NoError(t, PrepareEngine())
	assertSync(t, new(InsertMultipleMap))

	cnt, err = testEngine.Table(new(InsertMultipleMap)).Insert([]map[string]string{
		{
			"width":  "20",
			"height": "10",
			"name":   "lunny",
		},
		{
			"width":  "30",
			"height": "20",
			"name":   "xiaolunwen",
		},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 2, cnt)

	res = make([]InsertMultipleMap, 0, 2)
	err = testEngine.Find(&res)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(res))
	assert.EqualValues(t, InsertMultipleMap{
		Id:     1,
		Width:  20,
		Height: 10,
		Name:   "lunny",
	}, res[0])
	assert.EqualValues(t, InsertMultipleMap{
		Id:     2,
		Width:  30,
		Height: 20,
		Name:   "xiaolunwen",
	}, res[1])
}

func TestInsertNotDeleted(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	zeroTime := time.Date(1, 1, 1, 0, 0, 0, 0, testEngine.GetTZDatabase())
	type TestInsertNotDeletedStructNotRight struct {
		ID        uint64    `xorm:"'ID' pk autoincr"`
		DeletedAt time.Time `xorm:"'DELETED_AT' deleted notnull"`
	}
	// notnull tag will be ignored
	err := testEngine.Sync(new(TestInsertNotDeletedStructNotRight))
	assert.NoError(t, err)

	type TestInsertNotDeletedStruct struct {
		ID        uint64    `xorm:"'ID' pk autoincr"`
		DeletedAt time.Time `xorm:"'DELETED_AT' deleted"`
	}

	err = testEngine.Sync(new(TestInsertNotDeletedStruct))
	assert.NoError(t, err)

	var v1 TestInsertNotDeletedStructNotRight
	_, err = testEngine.Insert(&v1)
	assert.NoError(t, err)

	var v2 TestInsertNotDeletedStructNotRight
	has, err := testEngine.Get(&v2)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, v2.DeletedAt.In(testEngine.GetTZDatabase()).Format("2006-01-02 15:04:05"), zeroTime.Format("2006-01-02 15:04:05"))

	var v3 TestInsertNotDeletedStruct
	_, err = testEngine.Insert(&v3)
	assert.NoError(t, err)

	var v4 TestInsertNotDeletedStruct
	has, err = testEngine.Get(&v4)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, v4.DeletedAt.In(testEngine.GetTZDatabase()).Format("2006-01-02 15:04:05"), zeroTime.Format("2006-01-02 15:04:05"))
}

func TestInsertNotDeletedNum(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	type TestInsertNotDeletedNumStructNotRight struct {
		ID        uint64 `xorm:"'ID' pk autoincr"`
		DeletedAt int64  `xorm:"'DELETED_AT' deleted notnull INT(11)"`
	}
	// notnull tag will be ignored
	err := testEngine.Sync(new(TestInsertNotDeletedNumStructNotRight))
	assert.NoError(t, err)

	type TestInsertNotDeletedNumStruct struct {
		ID        uint64 `xorm:"'ID' pk autoincr"`
		DeletedAt int64  `xorm:"'DELETED_AT' deleted INT(11)"`
	}

	err = testEngine.Sync(new(TestInsertNotDeletedNumStruct))
	assert.NoError(t, err)

	var v1 TestInsertNotDeletedNumStructNotRight
	_, err = testEngine.Insert(&v1)
	assert.NoError(t, err)

	var v2 TestInsertNotDeletedNumStructNotRight
	has, err := testEngine.Get(&v2)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, v2.DeletedAt, int64(0))

	var v3 TestInsertNotDeletedNumStruct
	_, err = testEngine.Insert(&v3)
	assert.NoError(t, err)

	var v4 TestInsertNotDeletedNumStruct
	has, err = testEngine.Get(&v4)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, v4.DeletedAt, int64(0))
}

func TestInsertNotDeletedTimeStamp(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	// IN MYSQL DB
	// The time range that timestamps can store is from '1970 01 01 00:00:01.000000' to '2038 01 19 03:14:07.999999'
	// PASS notnull timestamp IN MYSQL DB
	if testEngine.Dialect().URI().DBType == schemas.MSSQL ||
		testEngine.Dialect().URI().DBType == schemas.SQLITE ||
		testEngine.Dialect().URI().DBType == schemas.POSTGRES {
		type TestInsertNotDeletedTimeStampStructNotRight struct {
			ID        uint64    `xorm:"'ID' pk autoincr"`
			DeletedAt time.Time `xorm:"'DELETED_AT' deleted notnull TIMESTAMP"`
		}
		err := testEngine.Sync(new(TestInsertNotDeletedTimeStampStructNotRight))
		assert.NoError(t, err)

		var v1 TestInsertNotDeletedTimeStampStructNotRight
		_, err = testEngine.Insert(&v1)
		assert.NoError(t, err)

		var v2 TestInsertNotDeletedTimeStampStructNotRight
		has, err := testEngine.Get(&v2)
		assert.NoError(t, err)
		assert.True(t, has)
		assert.Equal(t, v2.DeletedAt, time.Unix(0, 0))
	}

	type TestInsertNotDeletedTimeStampStruct struct {
		ID        uint64    `xorm:"'ID' pk autoincr"`
		DeletedAt time.Time `xorm:"'DELETED_AT' deleted TIMESTAMP"`
	}
	err := testEngine.Sync(new(TestInsertNotDeletedTimeStampStruct))
	assert.NoError(t, err)

	var v3 TestInsertNotDeletedTimeStampStruct
	_, err = testEngine.Insert(&v3)
	assert.NoError(t, err)

	var v4 TestInsertNotDeletedTimeStampStruct
	has, err := testEngine.Get(&v4)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, v4.DeletedAt, time.Time{})
}

type MyAutoTimeFields1 struct {
	Id int64
	Dt time.Time `xorm:"created DATETIME"`
}

func (MyAutoTimeFields1) TableName() string {
	return "my_auto_time_fields"
}

type MyAutoTimeFields2 struct {
	Id int64
	Dt time.Time `xorm:"created"`
}

func (MyAutoTimeFields2) TableName() string {
	return "my_auto_time_fields"
}

func TestAutoTimeFields(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	assertSync(t, new(MyAutoTimeFields1))

	_, err := testEngine.Insert(&MyAutoTimeFields1{})
	assert.NoError(t, err)

	var res []MyAutoTimeFields2
	assert.NoError(t, testEngine.Find(&res))
	assert.EqualValues(t, 1, len(res))

	_, err = testEngine.Insert(&MyAutoTimeFields2{})
	assert.NoError(t, err)

	res = []MyAutoTimeFields2{}
	assert.NoError(t, testEngine.Find(&res))
	assert.EqualValues(t, 2, len(res))
}

// insertIgnoreRecord exercises generated IDs, composite unique keys, quoted names,
// binary data and automatically populated timestamps together.
type insertIgnoreRecord struct {
	Id      int64     `xorm:"pk autoincr"`
	Group   int64     `xorm:"unique(ignore_key)"`
	Name    string    `xorm:"unique(ignore_key) varchar(100)"`
	Data    []byte    `xorm:"blob"`
	Created time.Time `xorm:"created"`
}

func TestInsertIgnore(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	dbType := testEngine.Dialect().URI().DBType
	if dbType != schemas.MYSQL && dbType != schemas.POSTGRES && dbType != schemas.SQLITE {
		t.Skip("InsertIgnore does not support this dialect")
	}
	version, err := testEngine.DBVersion()
	assert.NoError(t, err)
	t.Logf("InsertIgnore database: %s %v", dbType, version)
	assertSync(t, new(insertIgnoreRecord))

	first := insertIgnoreRecord{Group: 1, Name: "one", Data: []byte{0, 255, 39}}
	n, err := testEngine.InsertIgnore(&first)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.NotZero(t, first.Id)
	assert.False(t, first.Created.IsZero())

	duplicate := insertIgnoreRecord{Group: 1, Name: "one", Data: []byte("different")}
	n, err = testEngine.InsertIgnore(&duplicate)
	assert.NoError(t, err)
	assert.Zero(t, n)
	assert.Zero(t, duplicate.Id)
	assert.True(t, duplicate.Created.IsZero())

	rows := []insertIgnoreRecord{{Group: 1, Name: "one"}, {Group: 1, Name: "two"}, {Group: 1, Name: "two"}, {Group: 2, Name: "one"}}
	n, err = testEngine.InsertIgnore(rows)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, n)
	n, err = testEngine.InsertIgnore(&rows)
	assert.NoError(t, err)
	assert.Zero(t, n)
	for _, row := range rows {
		assert.Zero(t, row.Id, "batch insertion must not assign IDs")
	}

	pointers := []*insertIgnoreRecord{{Group: 1, Name: "two"}, {Group: 1, Name: "three"}}
	n, err = testEngine.InsertIgnore(pointers)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)
	n, err = testEngine.InsertIgnore([]any{&insertIgnoreRecord{Group: 1, Name: "three"}, &insertIgnoreRecord{Group: 1, Name: "four"}})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)

	var got insertIgnoreRecord
	found, err := testEngine.ID(first.Id).Get(&got)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, first.Data, got.Data)
	count, err := testEngine.Count(new(insertIgnoreRecord))
	assert.NoError(t, err)
	assert.EqualValues(t, 5, count)

	session := testEngine.NewSession()
	defer session.Close()
	assert.NoError(t, session.Begin())
	n, err = session.InsertIgnore(&insertIgnoreRecord{Group: 1, Name: "one"})
	assert.NoError(t, err)
	assert.Zero(t, n)
	n, err = session.Insert(&insertIgnoreRecord{Group: 1, Name: "after-ignore"})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.NoError(t, session.Commit())
	// Reusing a session must not turn subsequent ordinary inserts into ignores.
	_, err = session.Insert(&insertIgnoreRecord{Group: 1, Name: "one"})
	assert.Error(t, err)
}

func TestInsertIgnoreMaps(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	dbType := testEngine.Dialect().URI().DBType
	if dbType != schemas.MYSQL && dbType != schemas.POSTGRES && dbType != schemas.SQLITE {
		t.Skip("InsertIgnore does not support this dialect")
	}
	type insertIgnoreMap struct {
		Id   int64 `xorm:"pk"`
		Name string
	}
	assertSync(t, new(insertIgnoreMap))
	table := testEngine.TableName(new(insertIgnoreMap), true)
	n, err := testEngine.Table(table).InsertIgnore(map[string]any{"id": 1, "name": "original"})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)
	n, err = testEngine.Table(table).InsertIgnore([]map[string]any{{"id": 1, "name": "changed"}, {"id": 2, "name": "second"}, {"id": 2, "name": "changed"}})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)
	n, err = testEngine.Table(table).InsertIgnore(map[string]string{"id": "1", "name": "changed"})
	assert.NoError(t, err)
	assert.Zero(t, n)
	n, err = testEngine.Table(table).InsertIgnore([]map[string]string{{"id": "2", "name": "changed"}, {"id": "3", "name": "third"}})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)
	var got insertIgnoreMap
	found, err := testEngine.ID(1).Get(&got)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "original", got.Name)

	// A non-duplicate database error must still be returned and reset the session.
	session := testEngine.NewSession()
	defer session.Close()
	_, err = session.Table(table + "_missing").InsertIgnore(map[string]any{"id": 1})
	assert.Error(t, err)
	_, err = session.Table(table).Insert(map[string]any{"id": 1})
	assert.Error(t, err)
}

type insertIgnoreProcessor struct {
	Id      int64  `xorm:"pk autoincr"`
	Name    string `xorm:"unique varchar(100)"`
	Applied int    `xorm:"-"`
}

func (r *insertIgnoreProcessor) AfterInsert() { r.Applied++ }

func TestInsertIgnoreProcessors(t *testing.T) {
	assert.NoError(t, PrepareEngine())
	dbType := testEngine.Dialect().URI().DBType
	if dbType != schemas.MYSQL && dbType != schemas.POSTGRES && dbType != schemas.SQLITE {
		t.Skip("InsertIgnore does not support this dialect")
	}
	assertSync(t, new(insertIgnoreProcessor))
	first := &insertIgnoreProcessor{Name: "one"}
	n, err := testEngine.InsertIgnore(first)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.Equal(t, 1, first.Applied)
	ignored := &insertIgnoreProcessor{Name: "one"}
	n, err = testEngine.InsertIgnore(ignored)
	assert.NoError(t, err)
	assert.Zero(t, n)
	assert.Zero(t, ignored.Applied)
	assert.Zero(t, ignored.Id)
}
