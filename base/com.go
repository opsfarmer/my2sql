package base

import (
	"sync"
	"github.com/siddontang/go-mysql/mysql"
	//"github.com/siddontang/go-log/log"
	"github.com/siddontang/go-mysql/replication"
	"my2sql/dsql"
	//constvar "my2sql/constvar"
	//"fmt"
)

type BinEventHandlingIndx struct {
	EventIdx uint64
	lock     sync.RWMutex
	Finished bool
}

var (
	G_HandlingBinEventIndex *BinEventHandlingIndx
)

type MyBinEvent struct {
	MyPos       mysql.Position //this is the end position
	EventIdx    uint64
	BinEvent    *replication.RowsEvent
	StartPos    uint32 // this is the start position
	IfRowsEvent bool
	SqlType     string // insert, update, delete
	Timestamp   uint32
	TrxIndex    uint64
	TrxStatus   int           // 0:begin, 1: commit, 2: rollback, -1: in_progress
	QuerySql    *dsql.SqlInfo // for ddl and binlog which is not row format
	OrgSql      string        // for ddl and binlog which is not row format
}


func (this *MyBinEvent) CheckBinEvent(cfg *ConfCmd, ev *replication.BinlogEvent, currentBinlog *string) int {
	myPos := mysql.Position{Name: *currentBinlog, Pos: ev.Header.LogPos}

	if ev.Header.EventType == replication.ROTATE_EVENT {
		rotatEvent := ev.Event.(*replication.RotateEvent)
		*currentBinlog = string(rotatEvent.NextLogName)
		this.IfRowsEvent = false
		return C_reContinue
	}

	if cfg.IfSetStartFilePos {
		cmpRe := myPos.Compare(cfg.StartFilePos)
		if cmpRe == -1 {
			return C_reContinue
		}
	}

	if cfg.IfSetStopFilePos {
		cmpRe := myPos.Compare(cfg.StopFilePos)
		if cmpRe >= 0 {
			return C_reBreak
		}
	}
	//fmt.Println(cfg.StartDatetime, cfg.StopDatetime, header.Timestamp)
	if cfg.IfSetStartDateTime {
		if ev.Header.Timestamp < cfg.StartDatetime {
			return C_reContinue
		}
	}

	if cfg.IfSetStopDateTime {
		if ev.Header.Timestamp >= cfg.StopDatetime {
			return C_reBreak
		}
	}
	if cfg.FilterSqlLen == 0 {
		goto BinEventCheck
	}

	if ev.Header.EventType == replication.WRITE_ROWS_EVENTv1 || ev.Header.EventType == replication.WRITE_ROWS_EVENTv2 {
		if cfg.IsTargetDml("insert") {
			goto BinEventCheck
		} else {
			return C_reContinue
		}
	}

	if ev.Header.EventType == replication.UPDATE_ROWS_EVENTv1 || ev.Header.EventType == replication.UPDATE_ROWS_EVENTv2 {
		if cfg.IsTargetDml("update") {
			goto BinEventCheck
		} else {
			return C_reContinue
		}
	}

	if ev.Header.EventType == replication.DELETE_ROWS_EVENTv1 || ev.Header.EventType == replication.DELETE_ROWS_EVENTv2 {
		if cfg.IsTargetDml("delete") {
			goto BinEventCheck
		} else {
			return C_reContinue
		}
	}


	BinEventCheck:
	switch ev.Header.EventType {
	case replication.WRITE_ROWS_EVENTv1,
		replication.UPDATE_ROWS_EVENTv1,
		replication.DELETE_ROWS_EVENTv1,
		replication.WRITE_ROWS_EVENTv2,
		replication.UPDATE_ROWS_EVENTv2,
		replication.DELETE_ROWS_EVENTv2:

		wrEvent := ev.Event.(*replication.RowsEvent)
		db := string(wrEvent.Table.Schema)
		tb := string(wrEvent.Table.Table)
		if !cfg.IsTargetTable(db, tb) {
			return C_reContinue
		}
		/*
			if len(cfg.Databases) > 0 {
				if !sliceKits.ContainsString(cfg.Databases, db) {
					return C_reContinue
				}
			}
			if len(cfg.Tables) > 0 {
				if !sliceKits.ContainsString(cfg.Tables, tb) {
					return C_reContinue
				}
			}
		*/
		this.BinEvent = wrEvent
		this.IfRowsEvent = true
	case replication.QUERY_EVENT:
		this.IfRowsEvent = false


	case replication.XID_EVENT:
		this.IfRowsEvent = false

	case replication.MARIADB_GTID_EVENT:
		this.IfRowsEvent = false

	default:
		this.IfRowsEvent = false
		return C_reContinue
	}

	return C_reProcess

}