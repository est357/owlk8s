package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func main() {

	file, _ := os.Create("../eBPFprog.go")

	b, err := ioutil.ReadFile("http.o")
	if err != nil {
		fmt.Println("Could not read BPF object file", err.Error())
	}
	w := bufio.NewWriter(file)
	fileString := "package metrics\n\n func getEBPFProg() []byte { \n return []byte(\"" + binaryString(b) + "\") \n }\n"
	fmt.Fprint(w, fileString)
	w.Flush()
}

// From cilium :)
func binaryString(buf []byte) string {
	var builder strings.Builder
	for _, b := range buf {
		builder.WriteString(`\x`)
		builder.WriteString(fmt.Sprintf("%02x", b))
	}
	return builder.String()
}
