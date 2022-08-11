package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные

type (
	Column struct {
		Field      string
		Type       string
		Collation  sql.NullString
		Null       string
		Key        string
		Default    sql.NullString
		Extra      string
		Privileges string
		Comment    string
	}
	Col struct {
		Name string
		Type string
		Null bool
		PK   bool
	}
	Table struct {
		Columns       []Col
		PK            string
		AutoIncrement map[string]struct{}
		columnString  string
	}
	DbExplorer struct {
		db *sql.DB
		//regexps map[]
		tables  []string
		columns map[string]Table
	}
)

func (c *Col) defaultVar() interface{} {
	switch c.Type {
	case "varchar", "text":
		return ""
	case "int":
		return 0
	case "float64", "float":
		return 0.0
	}
	return ""
}

type finalResponse struct {
	Error    string                 `json:"error,omitempty"`
	Response map[string]interface{} `json:"response,omitempty"`
}

// нужно сохранить TABLES, поля в структуру!
func (d *DbExplorer) getAllTables() (tables []string, err error) {
	var rows *sql.Rows
	rows, err = d.db.Query("SHOW TABLES")
	if err != nil {
		return
	}
	defer rows.Close()
	var table string
	for rows.Next() {
		err = rows.Scan(&table)
		if err != nil {
			return
		}
		tables = append(tables, table)
	}
	return
}

func (d *DbExplorer) getColumns(table string) (tab Table, err error) {
	var rows *sql.Rows
	rows, err = d.db.Query("SHOW FULL COLUMNS FROM " + table)
	if err != nil {
		return
	}
	defer rows.Close()
	var col Column
	tab.Columns = make([]Col, 0)
	tab.AutoIncrement = make(map[string]struct{})
	pk := ""
	for rows.Next() {
		err = rows.Scan(&col.Field, &col.Type, &col.Collation, &col.Null, &col.Key, &col.Default, &col.Extra, &col.Privileges, &col.Comment)
		if err != nil {
			return
		}
		c := Col{
			Name: col.Field,
			Type: strings.Split(strings.ToLower(col.Type), "(")[0],
			Null: strings.ToLower(col.Null) == "yes",
			PK:   strings.ToLower(col.Key) == "pri",
		}
		if c.PK {
			pk = c.Name
		}
		tab.Columns = append(tab.Columns, c)
		if strings.ToLower(col.Extra) == "auto_increment" {
			tab.AutoIncrement[c.Name] = struct{}{}
		}
	}
	tab.PK = pk
	return
}

func (d *DbExplorer) processSelectRows(table string, rows *sql.Rows) (result []map[string]interface{}, err error) {
	result = make([]map[string]interface{}, 0)
	stubs := make([]interface{}, len(d.columns[table].Columns))
	stubsPtrs := make([]interface{}, len(d.columns[table].Columns))
	for i := range stubs {
		stubsPtrs[i] = &stubs[i]
	}
	for rows.Next() {
		err = rows.Scan(stubsPtrs...)
		if err != nil {
			return
		}
		res := make(map[string]interface{})
		for i, c := range d.columns[table].Columns {
			switch c.Type {
			case "int":
				v := &sql.NullInt32{}
				err = v.Scan(stubs[i])
				if err != nil {
					return
				}
				if c.Null && !v.Valid {
					res[c.Name] = nil
					break
				}
				res[c.Name] = v.Int32
			case "float":
				v := &sql.NullFloat64{}
				err = v.Scan(stubs[i])
				if err != nil {
					return
				}
				if c.Null && !v.Valid {
					res[c.Name] = nil
					break
				}
				res[c.Name] = v.Float64
			case "varchar", "text":
				v := &sql.NullString{}
				err = v.Scan(stubs[i])
				if err != nil {
					return
				}
				if c.Null && !v.Valid {
					res[c.Name] = nil
					break
				}
				res[c.Name] = v.String
			}
		}
		result = append(result, res)
	}
	return
}

func (d *DbExplorer) selectList(table string, limit, offset int) (result []map[string]interface{}, err error) {
	var rows *sql.Rows
	rows, err = d.db.Query(fmt.Sprintf("SELECT %s FROM %s LIMIT ? OFFSET ?", d.columns[table].columnString, table), limit, offset)
	if err != nil {
		return
	}
	defer rows.Close()
	return d.processSelectRows(table, rows)
}

func writeUnknownTable(w http.ResponseWriter) {
	resp := finalResponse{Error: "unknown table"}
	bs, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	w.Write(bs)
}

func writeRecordProblem(w http.ResponseWriter, err error) {
	resp := finalResponse{Error: err.Error()}
	bs, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	w.Write(bs)
}

func (d *DbExplorer) selectById(table string, id int) (result []map[string]interface{}, err error) {
	var rows *sql.Rows
	rows, err = d.db.Query(fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", d.columns[table].columnString, table, d.columns[table].PK), id)
	if err != nil {
		return
	}
	defer rows.Close()
	return d.processSelectRows(table, rows)
}

func extractPartsOfPath(r *http.Request) (arr []string) {
	for _, s := range strings.Split(r.URL.Path, "/") {
		if s != "" {
			arr = append(arr, s)
		}
	}
	return
}

func (d *DbExplorer) createRecord(table string, rawRecord map[string]interface{}) (result map[string]interface{}, err error) {
	result = make(map[string]interface{})
	for _, col := range d.columns[table].Columns {
		v, ok := rawRecord[col.Name]
		if !ok {
			continue
		}
		ttype := fmt.Sprintf("%T", v)
		misTypes := ttype == "int" && col.Type != "int" || ttype == "float64" && col.Type != "int" && col.Type != "float64" || ttype == "string" && col.Type != "varchar" && col.Type != "text"
		nullCheck := v == nil && !col.Null
		if misTypes || nullCheck {
			return result, errors.New(fmt.Sprintf("field %s have invalid type", col.Name))
		}
		if col.Type == "int" {
			result[col.Name] = int(v.(float64))
			continue
		}
		result[col.Name] = v
	}
	return result, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func writeResponse(w http.ResponseWriter, resp finalResponse) {
	bs, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(bs)
}

func (d *DbExplorer) updateRecord(table string, id int, record map[string]interface{}) (updated int, err error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	cols := make([]string, 0)
	vals := make([]interface{}, 0)
	pk := d.columns[table].PK
	for k, v := range record {
		if _, ok := d.columns[table].AutoIncrement[k]; ok {
			continue
		}
		cols = append(cols, fmt.Sprintf("%s = ?", k))
		vals = append(vals, v)
	}
	vals = append(vals, id)
	colsString := strings.Join(cols, ", ")
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", table, colsString, pk)

	res, err := tx.Exec(q, vals...)
	if err != nil {
		return 0, err
	}
	err = tx.Commit()
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	return boolToInt(affected > 0), err
}

func (d *DbExplorer) deleteById(table string, id int) (deleted int, err error) {
	q := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", table, d.columns[table].PK)
	res, err := d.db.Exec(q, id)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return boolToInt(affected > 0), err
}

func (d *DbExplorer) insertRecord(table string, record map[string]interface{}) (lastId int64, err error) {
	tx, err := d.db.Begin()
	if err != nil {
		return lastId, err
	}
	defer tx.Rollback()

	cols := make([]string, 0)
	vals := make([]interface{}, 0)
	questions := make([]string, 0)
	for _, c := range d.columns[table].Columns {
		if _, ok := d.columns[table].AutoIncrement[c.Name]; ok {
			continue
		}

		v, ok := record[c.Name]
		if !ok && c.Null {
			continue
		}
		cols = append(cols, c.Name)
		if !ok {
			v = c.defaultVar()
		}
		vals = append(vals, v)
		questions = append(questions, "?")
	}
	colsString := strings.Join(cols, ", ")
	questionsString := strings.Join(questions, ", ")
	q := fmt.Sprintf("INSERT INTO %s(%s) VALUES (%s)", table, colsString, questionsString)

	res, err := tx.Exec(q, vals...)
	if err != nil {
		return lastId, err
	}
	lastId, err = res.LastInsertId()
	if err != nil {
		return lastId, err
	}
	err = tx.Commit()
	return lastId, err
}

func readParam(r *http.Request, paramName string, defaultValue int) int {
	if param := r.URL.Query().Get(paramName); len(param) > 0 {
		tmp, err := strconv.Atoi(param)
		if err != nil {
			return defaultValue
		}
		if tmp < 0 {
			return defaultValue
		}
		return tmp
	}
	return defaultValue
}

func (d *DbExplorer) getTables(w http.ResponseWriter) {
	resp := finalResponse{Response: map[string]interface{}{"tables": d.tables}}
	writeResponse(w, resp)
}

func (d *DbExplorer) getFromTable(w http.ResponseWriter, r *http.Request, arr []string) {
	table := arr[0]
	_, ok := d.columns[table]
	if !ok {
		writeUnknownTable(w)
		return
	}

	limit := readParam(r, "limit", 5)
	offset := readParam(r, "offset", 0)

	result, err := d.selectList(table, limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := finalResponse{Response: map[string]interface{}{"records": result}}
	writeResponse(w, resp)
}

func (d *DbExplorer) getRecord(w http.ResponseWriter, arr []string) {
	var err error
	var resp finalResponse
	table := arr[0]
	idString := arr[1]
	var id int

	id, err = strconv.Atoi(idString)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, ok := d.columns[table]
	if !ok {
		writeUnknownTable(w)
		return
	}

	result, err := d.selectById(table, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(result) == 0 {
		resp = finalResponse{Error: "record not found"}
		bs, err := json.Marshal(resp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write(bs)
		return
	}

	resp = finalResponse{Response: map[string]interface{}{"record": result[0]}}
	writeResponse(w, resp)
}

func (d *DbExplorer) putRecord(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp finalResponse
	table := ""
	if arr := extractPartsOfPath(r); len(arr) > 0 {
		table = arr[0]
	}

	_, ok := d.columns[table]
	if !ok {
		writeUnknownTable(w)
		return
	}

	defer r.Body.Close()
	bs, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var rawRecord map[string]interface{}
	err = json.Unmarshal(bs, &rawRecord)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	record, err := d.createRecord(table, rawRecord)
	if err != nil {
		writeRecordProblem(w, err)
		return
	}
	lastId, err := d.insertRecord(table, record)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = finalResponse{Response: map[string]interface{}{d.columns[table].PK: lastId}}
	writeResponse(w, resp)
}

func (d *DbExplorer) postRecord(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp finalResponse
	table := ""
	idString := ""
	var id int
	if arr := extractPartsOfPath(r); len(arr) == 2 {
		table = arr[0]
		idString = arr[1]
		id, err = strconv.Atoi(idString)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, ok := d.columns[table]
	if !ok {
		writeUnknownTable(w)
		return
	}

	defer r.Body.Close()
	bs, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var rawRecord map[string]interface{}
	err = json.Unmarshal(bs, &rawRecord)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for k := range rawRecord {
		if _, ok := d.columns[table].AutoIncrement[k]; ok {
			err = errors.New(fmt.Sprintf("field %s have invalid type", d.columns[table].PK))
			writeRecordProblem(w, err)
			return
		}
	}
	record, err := d.createRecord(table, rawRecord)
	if err != nil {
		writeRecordProblem(w, err)
		return
	}

	updated, err := d.updateRecord(table, id, record)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = finalResponse{Response: map[string]interface{}{"updated": updated}}
	writeResponse(w, resp)
}

func (d *DbExplorer) deleteRecord(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp finalResponse
	table := ""
	idString := ""
	if arr := extractPartsOfPath(r); len(arr) > 0 {
		table = arr[0]
		idString = arr[1]
	}

	_, ok := d.columns[table]
	if !ok {
		writeUnknownTable(w)
		return
	}

	id, err := strconv.Atoi(idString)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	deleted, err := d.deleteById(table, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = finalResponse{Response: map[string]interface{}{"deleted": deleted}}
	writeResponse(w, resp)
}

func (d *DbExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//slashCount := strings.Count(r.URL.Path, "/")
	switch {
	case r.Method == "GET" && r.URL.Path == "/":
		d.getTables(w)
	case r.Method == "GET":
		arr := extractPartsOfPath(r)
		if len(arr) == 1 {
			d.getFromTable(w, r, arr)
		} else if len(arr) == 2 {
			//case r.Method == "GET" && slashCount == 2:
			d.getRecord(w, arr)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case r.Method == "PUT":
		d.putRecord(w, r)
	case r.Method == "POST":
		d.postRecord(w, r)
	case r.Method == "DELETE":
		d.deleteRecord(w, r)
	}
}

func NewDbExplorer(db *sql.DB) (d *DbExplorer, err error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	d = &DbExplorer{db: db}
	d.tables, err = d.getAllTables()
	d.columns = make(map[string]Table, len(d.tables))
	for _, table := range d.tables {
		tab, err := d.getColumns(table)
		if err != nil {
			return d, err
		}
		cols := make([]string, len(tab.Columns))
		for i, c := range tab.Columns {
			cols[i] = c.Name
		}
		tab.columnString = strings.Join(cols, ", ")
		d.columns[table] = tab
	}
	return d, nil
}
