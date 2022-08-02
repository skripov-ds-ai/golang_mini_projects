package main

//  go build handlers_gen/* && ./codegen api.go api_handlers.go

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"reflect"
	"strings"
	"text/template"
)

type modelDeserializer struct {
	ModelType         string
	UnpackFieldsBlock string
}

type apiConfig struct {
	URL    string `json:"url"`
	Auth   bool   `json:"auth"`
	Method string `json:"method"`
}

type handlerConfig struct {
	ApiType              string
	ApiMethod            string
	ParamsType           string
	CheckAuthBlock       string
	CheckHttpMethodBlock string
}

type handlerApiConfig struct {
	ApiMethod string
	URL       string
}

var (
	packagesArr = []string{
		"fmt",
		"strconv",
		"net/http",
		"encoding/json",
	}

	handlerTpl = template.Must(template.New("handlerTpl").Parse(`
func (a *{{.ApiType}}) handler{{.ApiMethod}}(w http.ResponseWriter, r *http.Request) {
	var err error{{.CheckHttpMethodBlock}}{{.CheckAuthBlock}}
	params, err := unpack{{.ParamsType}}(r)
	if err != nil {
		handleError(w, err)
		return
	}
	resp, err := a.{{.ApiMethod}}(r.Context(), params)
	if err != nil {
		handleError(w, err)
		return
	}
	bs, err := json.Marshal(
		finalResponse{
			Response: resp,
		},
	)
	if err != nil {
		handleError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(bs)
}
`))

	unpack = template.Must(template.New("model").Parse(`
func unpack{{.ModelType}}(r *http.Request) (m {{.ModelType}}, err error) {
	{{.UnpackFieldsBlock}}
	return m, nil
}
`))

	checkingAuth = `
	// checking authentication
	err = checkAuth(r)
	if err != nil {
		handleError(w, err)
		return
	}`

	helpers = `
type finalResponse struct {
	Error string ` + "`json:\"error\"`" + `
	Response interface{} ` + "`json:\"response,omitempty\"`" + `
}

func checkAuth(r *http.Request) error {
	if r.Header.Get("X-Auth") != "100500" {
		return ApiError{http.StatusForbidden, fmt.Errorf("unauthorized")}
	}
	return nil
}

func handleError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if apiError, ok := err.(ApiError); ok {
		status = apiError.HTTPStatus
	}
	bs, e := json.Marshal(
		finalResponse{
			Error:    err.Error(),
			Response: nil,
		},
	)
	if e != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	w.Write(bs)
}

func extractValueFromRequest(r *http.Request, name string) (string, error) {
	var val string
	if r.Method == "GET" {
		val = r.URL.Query().Get(name)
	} else if r.Method == "POST" {
		val = r.FormValue(name)
	} else {
		return val, fmt.Errorf("Unsupported http method")
	}
	return val, nil
}
`
)

func createHandler(out io.Writer, funcDecl *ast.FuncDecl, config *apiConfig) handlerConfig {
	hConfig := handlerConfig{}
	hConfig.ApiMethod = funcDecl.Name.Name
	hConfig.ParamsType = funcDecl.Type.Params.List[1].Type.(*ast.Ident).Name
	hConfig.ApiType = funcDecl.Recv.List[0].Type.(*ast.StarExpr).X.(*ast.Ident).Name

	fmt.Printf("Generating handler for %s.%s(%s)\n", hConfig.ApiType, hConfig.ApiMethod, hConfig.ParamsType)

	if config.Auth {
		hConfig.CheckAuthBlock = checkingAuth
	}
	if config.Method != "" {
		hConfig.CheckHttpMethodBlock = `
	// checking http method correctness
	if r.Method != "` + config.Method + `" {
		handleError(w, ApiError{http.StatusNotAcceptable, fmt.Errorf("bad method")})
		return
	}`
	}
	handlerTpl.Execute(out, hConfig)
	return hConfig
}

func createUnpackFieldsBlock(name, ttype, tag string) string {
	var buff bytes.Buffer
	options := make(map[string]string)

	if tag != "" {
		for _, optionInfo := range strings.Split(tag, ",") {
			infos := strings.Split(optionInfo, "=")
			if size := len(infos); size > 0 && size < 3 {
				val := ""
				if size == 2 {
					val = infos[1]
				}
				options[infos[0]] = val
			} else {
				panic("Unsupported tag for a field")
			}
		}
	}

	newName, ok := options["paramname"]
	if !ok {
		newName = strings.ToLower(name)
	}

	buff.WriteString(`
	val, err = extractValueFromRequest(r, "` + newName + `")
	if err != nil {
		return m, ApiError{http.StatusBadRequest, err}
	}`)

	if defaultVal, ok := options["default"]; ok {
		defaultString := defaultVal
		if ttype == "string" {
			defaultString = fmt.Sprintf("\"%s\"", defaultString)
		}
		buff.WriteString(`
	if val == "" {
		val = ` + defaultString + `
	}`)
	}

	if _, ok = options["required"]; ok {
		buff.WriteString(`
	if val == "" {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("` + newName + ` must me not empty")}
	}`)
	}

	if ttype == "string" {
		if enumVal, ok := options["enum"]; ok {
			enumVals := strings.Split(enumVal, "|")
			quotedEnumVals := make([]string, len(enumVals))
			for i, enum := range enumVals {
				quotedEnumVals[i] = `"` + enum + `"`
			}
			buff.WriteString(`
	switch val {
	case ` + strings.Join(quotedEnumVals, ", ") + `:
	default:
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("` + newName + " must be one of [" + strings.Join(enumVals, ", ") + `]")}
	}`)
		}
		buff.WriteString(`
	m.` + name + ` = val`)
	} else if ttype == "int" {
		buff.WriteString(`
	var conversionErr error
	m.` + name + `, conversionErr = strconv.Atoi(val)
	if conversionErr != nil {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("` + newName + ` must be int")}
	}`)
	}

	if min, ok := options["min"]; ok {
		if ttype == "string" {
			buff.WriteString(`
	if len(m.` + name + ") < " + min + ` {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("` + newName + ` len must be >= ` + min + `")}
	}`)
		} else if ttype == "int" {
			buff.WriteString(`
	if m.` + name + " < " + min + ` {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("` + newName + ` must be >= ` + min + `")}
	}`)
		}
	}

	if max, ok := options["max"]; ok {
		if ttype == "string" {
			buff.WriteString(`
	if len(m.` + name + ") > " + max + ` {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("` + newName + ` len must be <= ` + max + `")}
	}`)
		} else if ttype == "int" {
			buff.WriteString(`
	if m.` + name + " > " + max + ` {
		return m, ApiError{http.StatusBadRequest, fmt.Errorf("` + newName + ` must be <= ` + max + `")}
	}`)
		}
	}

	return buff.String()
}

func createUnpackers(out io.Writer, node *ast.File, paramsInfo map[string]struct{}) {
	for _, f := range node.Decls {
		g, ok := f.(*ast.GenDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.GenDecl\n", f)
			continue
		}
		for _, spec := range g.Specs {
			ttype, ok := spec.(*ast.TypeSpec)
			if !ok {
				fmt.Printf("SKIP %T is not ast.TypeSpec\n", spec)
				continue
			}

			typeName := ttype.Name.Name
			if _, ok = paramsInfo[typeName]; !ok {
				fmt.Printf("SKIP %s is not a type of handler's parameter\n", typeName)
				continue
			}

			currStruct, ok := ttype.Type.(*ast.StructType)
			if !ok {
				fmt.Printf("SKIP %T is not ast.StructType\n", currStruct)
				continue
			}
			createUnpacker(out, currStruct, typeName)
		}
	}
}

func createUnpacker(out io.Writer, currStruct *ast.StructType, structName string) {
	var buff bytes.Buffer
	buff.WriteString(`var val string`)
	for _, field := range currStruct.Fields.List {
		if field.Tag != nil {
			name := field.Names[0].Name
			ttype := field.Type.(*ast.Ident).Name
			tagValue := field.Tag.Value
			tagInfo := reflect.StructTag(tagValue[1 : len(tagValue)-1])
			tag := tagInfo.Get("apivalidator")

			fmt.Printf("Generating deserialization and validation for field %s with tag %s\n", name, tag)

			buff.WriteString(createUnpackFieldsBlock(name, ttype, tag))
			//buff.WriteRune('\n')
		}
	}
	unpack.Execute(out, modelDeserializer{ModelType: structName, UnpackFieldsBlock: buff.String()})
}

func createServeHttp(out io.Writer, apiType string, configs []handlerApiConfig) {
	fmt.Fprint(out, `
func (h *`+apiType+`) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {`)
	for _, config := range configs {
		fmt.Fprint(out, fmt.Sprintf("\n\tcase \"%s\":\n\t\th.handler%s(w, r)", config.URL, config.ApiMethod))
	}
	fmt.Fprint(out, `
	default:
		handleError(w, ApiError{http.StatusNotFound, fmt.Errorf("unknown method")})`,
	)
	fmt.Fprint(out, "\n\t}\n}\n")
}

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, "import (")
	for _, pack := range packagesArr {
		fmt.Fprintf(out, "\t\"%s\"\n", pack)
	}
	fmt.Fprintln(out, ")\n") // empty line
	fmt.Fprintln(out, helpers)

	paramsInfo := make(map[string]struct{})
	hConfigs := make(map[string][]handlerApiConfig)
	for _, f := range node.Decls {
		funcDecl, ok := f.(*ast.FuncDecl)
		if !ok || funcDecl.Doc == nil {
			continue
		}

		needCodegen := false
		var config apiConfig
		for _, comment := range funcDecl.Doc.List {
			needCodegen = needCodegen || strings.HasPrefix(comment.Text, "// apigen:api")
			configStr := strings.Replace(comment.Text, "// apigen:api ", "", 1)
			config = apiConfig{}
			json.Unmarshal([]byte(configStr), &config)
		}
		fmt.Printf("config = %v\n", config)

		if needCodegen {
			hConfig := createHandler(out, funcDecl, &config)
			paramsInfo[hConfig.ParamsType] = struct{}{}
			hConfigs[hConfig.ApiType] = append(
				hConfigs[hConfig.ApiType],
				handlerApiConfig{ApiMethod: hConfig.ApiMethod, URL: config.URL},
			)
		}
	}

	createUnpackers(out, node, paramsInfo)
	for apiType, apiHandler := range hConfigs {
		createServeHttp(out, apiType, apiHandler)
	}
}
