package main

import (
	"bytes"
	"fmt"
	"github.com/mailru/easyjson"
	user2 "golang_mini_projects/optimization/user"
	"io"
	"os"
	"regexp"
	"strings"
)

const filePath1 string = "./data/users1.txt"

// вам надо написать более быструю оптимальную этой функции
func FastSearch(out io.Writer) {
	file, err := os.Open(filePath1)
	defer file.Close()
	if err != nil {
		panic(err)
	}

	//fileContents, err := ioutil.ReadAll(file)
	//if err != nil {
	//	panic(err)
	//}
	var buf bytes.Buffer
	_, err = io.Copy(&buf, file)
	if err != nil {
		panic(err)
	}
	fileContents := buf.Bytes()

	r := regexp.MustCompile("@")
	android := regexp.MustCompile("Android")
	msie := regexp.MustCompile("MSIE")
	//seenBrowsers := []string{}
	seenBrowsers := make(map[string]struct{})
	//seenBrowsersNum := 0
	//uniqueBrowsers := 0
	foundUsers := ""
	i := -1

	lines := strings.Split(string(fileContents), "\n")

	//users := make([]map[string]interface{}, 0, len(lines))
	for _, line := range lines {
		user := user2.User{}
		//user := make(map[string]interface{})
		// fmt.Printf("%v %v\n", err, line)
		err = easyjson.Unmarshal([]byte(line), &user)
		//err := json.Unmarshal([]byte(line), &user)
		if err != nil {
			panic(err)
		}
		i++

		isAndroid := false
		isMSIE := false

		browsers := user.Browsers
		for _, browser := range browsers {
			if ok := android.MatchString(browser); ok {
				isAndroid = true
				_, ok1 := seenBrowsers[browser]
				if !ok1 {
					seenBrowsers[browser] = struct{}{}
				}
			} else if ok = msie.MatchString(browser); ok {
				isMSIE = true
				_, ok1 := seenBrowsers[browser]
				if !ok1 {
					seenBrowsers[browser] = struct{}{}
				}
			}
		}

		if !(isAndroid && isMSIE) {
			continue
		}

		// log.Println("Android and MSIE user:", user["name"], user["email"])
		email := r.ReplaceAllString(user.Email, " [at] ")
		foundUsers += fmt.Sprintf("[%d] %s <%s>\n", i, user.Name, email)
	}

	fmt.Fprintln(out, "found users:\n"+foundUsers)
	fmt.Fprintln(out, "Total unique browsers", len(seenBrowsers))
}
