package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type OSEntries []os.DirEntry

func (a OSEntries) Len() int {
	return len(a)
}

func (a OSEntries) Less(i, j int) bool {
	return a[i].Name() > a[j].Name()
}

func (a OSEntries) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func FilterOSEntries(oldEntries OSEntries) (entries OSEntries) {
	for _, entry := range oldEntries {
		if entry.IsDir() {
			entries = append(entries, entry)
		}
	}
	return
}

func GetInitPathInfo(path string) (isDir bool, err error) {
	var f *os.File
	f, err = os.Open(path)
	defer f.Close()
	if err != nil {
		return
	}
	var stat os.FileInfo
	stat, err = f.Stat()
	isDir = stat.IsDir()
	return
}

func GetOSEntries(path string) (entries OSEntries, err error) {
	var f *os.File
	f, err = os.Open(path)
	defer f.Close()
	if err != nil {
		return
	}
	entries, err = f.ReadDir(0)
	return
}

func GetFileSizeString(path string) (res string, err error) {
	var f *os.File
	f, err = os.Open(path)
	defer f.Close()
	if err != nil {
		return
	}
	var stat os.FileInfo
	stat, err = f.Stat()
	if err != nil {
		return
	}
	size := stat.Size()
	if size == 0 {
		return " (empty)", nil
	}
	res = fmt.Sprintf(" (%db)", size)
	return
}

const (
	ordinaryBranch = "├───"
	lastBranch     = "└───"
	osSeparator    = string(os.PathSeparator)
	tabSeparator   = "\t"
	newString      = "\n"
	vertical       = "│"
)

type PrintInfo struct {
	Pre            string
	IsLast         bool
	FullPath       string
	IsDir          bool
	VerticalNumber int
}

func GetFinalStringForNode(node PrintInfo) (res string, err error) {
	s := strings.Split(node.FullPath, osSeparator)
	last := ""
	branch := ordinaryBranch
	if node.IsLast {
		branch = lastBranch
	}
	if !node.IsDir {
		last, err = GetFileSizeString(node.FullPath)
		if err != nil {
			return
		}
	}
	return node.Pre + branch + s[len(s)-1] + last + newString, nil
}

func PrintNode(out io.Writer, node PrintInfo) (err error) {
	finalString, err := GetFinalStringForNode(node)
	var bs = []byte(finalString)
	out.Write(bs)
	return err
}

func dirTree(out io.Writer, path string, printFiles bool) (err error) {
	var stack []PrintInfo
	var isDir bool

	isDir, err = GetInitPathInfo(path)
	var startNode = PrintInfo{
		IsLast:   false,
		FullPath: path,
		IsDir:    isDir,
	}
	stack = append(stack, startNode)
	isFirstNode := true

	for len(stack) > 0 {
		var entries OSEntries
		var node PrintInfo
		var infos []PrintInfo

		idx := len(stack) - 1
		node, stack = stack[idx], stack[:idx]
		if node.IsDir {
			entries, err = GetOSEntries(node.FullPath)
			if err != nil {
				return
			}
			if !printFiles {
				entries = FilterOSEntries(entries)
			}
			sort.Sort(entries)
			for i, entry := range entries {
				tmp := PrintInfo{
					IsLast:   i == 0,
					FullPath: node.FullPath + osSeparator + entry.Name(),
					IsDir:    entry.IsDir(),
					Pre:      node.Pre,
				}
				if !isFirstNode {
					if !node.IsLast {
						tmp.Pre += vertical
					}
					tmp.Pre += tabSeparator
				}
				infos = append(infos, tmp)
			}
			stack = append(stack, infos...)
		}
		if !isFirstNode {
			err = PrintNode(out, node)
			if err != nil {
				return
			}
		} else {
			isFirstNode = false
		}
	}
	return
}

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}
