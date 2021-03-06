// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tool

import (
	"database/sql"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"net/url"

	"k8s.io/klog"
)

type MysqlService struct {
	db  *sql.DB
	dsn string
}

type MysqlKubeEventPoint struct {
	Namespace   string
	Kind        string
	Name        string
	Type        string
	Reason      string
	Message     string
	TriggerTime string
	CreateTime  string
}

func (mySvc MysqlService) SaveData(sinkData []interface{}) error {

	if len(sinkData) == 0 {
		klog.Warningf("insert data is []")
		return nil
	}

	_, err := mysql.ParseDSN(mySvc.dsn)
	if err != nil {
		klog.Errorf("failed to Parse DSN")
		return err
	}

	statementDBName := "kube_pod_event"

	prepareStatement := fmt.Sprintf("INSERT INTO %s (namespace,kind,name,type,reason,message,trigger_time) VALUES(?,?,?,?,?,?,?)", statementDBName)

	// Prepare statement for inserting data
	stmtIns, err := mySvc.db.Prepare(prepareStatement)
	if err != nil {
		klog.Errorf("failed to Prepare statement for inserting data ")
		return err
	}

	defer stmtIns.Close()

	for _, data := range sinkData {

		ked := data.(MysqlKubeEventPoint)
		klog.Infof("Begin Insert Mysql Data ...")
		klog.Infof("Namespace: %s, Kind: %s, Name: %s, Type: %s, Reason: %s, Message: %s, TriggerTime: %s ", ked.Namespace, ked.Kind, ked.Name, ked.Type, ked.Reason, ked.Message, ked.TriggerTime)
		_, err = stmtIns.Exec(ked.Namespace, ked.Kind, ked.Name, ked.Type, ked.Reason, ked.Message, ked.TriggerTime)
		if err != nil {
			klog.Errorf("failed to Prepare statement for inserting data ")
			return err
		}
		klog.Infof("Insert Mysql Data Suc...")

	}

	return nil
}

func (mySvc MysqlService) FlushData() error {
	return nil
}

func (mySvc MysqlService) CreateDatabase(name string) error {
	return nil
}

func (mySvc MysqlService) CloseDB() error {
	return mySvc.db.Close()
}

func NewMysqlClient(uri *url.URL) (*MysqlService, error) {

	mysqlSvc := MysqlService{
		dsn: uri.RawQuery,
	}

	klog.Infof("mysql jdbc url: %s", mysqlSvc.dsn)

	db, err := sql.Open("mysql", mysqlSvc.dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect mysql according jdbc url string: %s", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)

	// Open doesn't open a connection. Validate DSN data:
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("cannot open a connection for mysql according jdbc url string: %s", err)
	}

	mysqlSvc.db = db

	return &mysqlSvc, nil
}
