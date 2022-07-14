package main

import (
	"fmt"
	"io"
	"os"
	"sort"
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
	f, err := os.Open(path)
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
	f, err := os.Open(path)
	defer f.Close()
	if err != nil {
		return
	}
	entries, err = f.ReadDir(0)
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
	Pre      string
	IsLast   bool
	FullPath string
	IsDir    bool
	Size     int64
	Name     string
}

func GetFinalStringForNode(node PrintInfo) (res string) {
	last := ""
	branch := ordinaryBranch
	if node.IsLast {
		branch = lastBranch
	}
	if !node.IsDir {
		if size := node.Size; size != 0 {
			last = fmt.Sprintf(" (%db)", size)
		} else {
			last = " (empty)"
		}
	}
	return node.Pre + branch + node.Name + last + newString
}

func PrintNode(out io.Writer, node PrintInfo) {
	finalString := GetFinalStringForNode(node)
	var bs = []byte(finalString)
	out.Write(bs)
}

func dirTree(out io.Writer, path string, printFiles bool) (err error) {
	isDir, err := GetInitPathInfo(path)
	if err != nil {
		return
	}

	var stack []PrintInfo
	startNode := PrintInfo{
		IsLast:   false,
		FullPath: path,
		IsDir:    isDir,
	}
	stack = append(stack, startNode)
	isFirstNode := true

	for len(stack) > 0 {
		var node PrintInfo

		idx := len(stack) - 1
		node, stack = stack[idx], stack[:idx]

		if node.IsDir {
			entries, err := GetOSEntries(node.FullPath)
			if err != nil {
				return err
			}

			if !printFiles {
				entries = FilterOSEntries(entries)
			}

			sort.Sort(entries)

			for i, entry := range entries {
				name := entry.Name()
				tmp := PrintInfo{
					IsLast:   i == 0,
					FullPath: node.FullPath + osSeparator + name,
					IsDir:    entry.IsDir(),
					Pre:      node.Pre,
					Name:     name,
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				tmp.Size = info.Size()

				if !isFirstNode {
					if !node.IsLast {
						tmp.Pre += vertical
					}
					tmp.Pre += tabSeparator
				}
				stack = append(stack, tmp)
			}
		}

		if isFirstNode {
			isFirstNode = false
			continue
		}
		PrintNode(out, node)
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
