package main

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
)

type Calls struct {
	Name        string
	Count       int
	RoutineType string
}

const (
	mb   = 1 << 10
	size = 250 * mb
)

/*
export FILE=routines-trace.txt && ./goan split && ./goan > stat.txt && ./goan clean
*/
func main() {
	if len(os.Args) < 1 {
		panic("no args")
	}
	fmt.Println(os.Args)
	filename := strings.TrimSpace(os.Getenv("FILE"))
	if filename == "" {
		panic("empty FILE env")
	}
	fs, err := os.Stat(filename)
	if err != nil {
		panic("filename:" + filename + " err=" + err.Error())
	}
	parts := int(fs.Size()/size) + 1
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	if len(os.Args) > 1 && os.Args[1] == "clean" {
		for i := 0; i < parts; i++ {
			name := fmt.Sprintf("%s_%d.txt", strings.TrimSuffix(filename, ".txt"), i+1)
			if err = os.Remove(name); err != nil {
				panic(err)
			}
		}

		return
	}

	if len(os.Args) > 1 && os.Args[1] == "split" {
		var buf [size]byte
		for i := 0; i < parts; i++ {
			n, err := file.Read(buf[:])
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}
			name := fmt.Sprintf("%s_%d.txt", strings.TrimSuffix(filename, ".txt"), i+1)
			if _, err := os.Stat(name); err == nil {
				_ = os.Remove(name)
			}
			if err = os.WriteFile(name, buf[:n], 0666); err != nil {
				panic(err)
			}
		}

		return
	}

	blockingTypesStats := make(map[string]int)
	type block struct {
		tpe, name string
	}
	blocks := make([]block, 0)
	for i := 0; i < parts; i++ {
		name := fmt.Sprintf("%s_%d.txt", strings.TrimSuffix(filename, ".txt"), i+1)
		file, err := os.Open(name)
		if err != nil {
			panic(err)
		}
		scanner := bufio.NewScanner(file)

		var blockStarted = true
		goroutineType := ""
		for scanner.Scan() {
			text := scanner.Text()
			if strings.Contains(text, "goroutine") && strings.Contains(text, ":") {
				parts := strings.SplitN(text, " ", 3)
				if len(parts) != 3 {

					continue
				}
				goroutineType = parts[2]
				blockingTypesStats[goroutineType[1:len(goroutineType)-2]]++

				continue
			}
			if len(strings.TrimSpace(text)) == 0 {
				blockStarted = true
				continue
			}
			if !blockStarted {
				continue
			}
			blockStarted = false
			parts := strings.Split(text, " ")
			block := block{}
			if goroutineType != "" {
				block.tpe = goroutineType
			}
			if len(parts) <= 1 {
				block.name = parts[0]
			}

			blocks = append(blocks, block)
		}
	}
	stats := make(map[block]int)

	for _, v := range blocks {
		if idx := strings.Index(v.name, "0x"); idx != -1 {
			if idx != 0 && idx-1 < len(v.name) {
				idx -= 1
			}
			v.name = v.name[:idx]
		}
		stats[v]++
	}
	var stat = make([]Calls, 0, len(stats))
	for k, v := range stats {
		stat = append(stat, Calls{
			Name:        k.name,
			RoutineType: k.tpe,
			Count:       v,
		})
	}
	slices.SortFunc(stat, func(a, b Calls) int {
		return cmp.Compare(b.Count, a.Count)
	})
	tabwriter := tabwriter.NewWriter(os.Stdout, 1, 8, 1, '\t', 0)
	subByType := 0
	for k, v := range blockingTypesStats {
		_, _ = fmt.Fprintln(tabwriter, k, v)
		subByType += v
	}
	_, _ = fmt.Fprintf(tabwriter, "SUM: %d\n", subByType)
	subByName := 0
	buf := bytes.Buffer{}
	for _, v := range stat {
		var w io.Writer = tabwriter
		if v.RoutineType == "" || v.Name == "" {
			w = &buf
		}
		_, _ = fmt.Fprintln(w, v.RoutineType, "|", v.Name, "|", v.Count)
		subByName += v.Count
	}
	_, _ = tabwriter.Write(buf.Bytes())
	_, _ = fmt.Fprintf(tabwriter, "SUM: %d\n", subByName)
}
