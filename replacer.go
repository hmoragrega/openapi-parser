package openapiparser

/*import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

var refKey = []byte("$ref:")
var hash = []byte("#")

// replaceReferences will load in memory all referenced files
// recursively and replace the reference to the key of the
// returned data.
func ReplaceReferences(file string) map[string][]byte {
	files := make(map[string][]byte)
	readFile(file, files)

	fmt.Println(string(files[file]))

	return files
}

func readFile(file string, cache map[string][]byte) []byte {
	file, err := filepath.Abs(file)
	if err != nil {
		panic(fmt.Errorf("cannot calculate absolute path %q: %w", file, err))
	}
	f, err := os.Open(file)
	if err != nil {
		panic(fmt.Errorf("cannot open file %q: %w", file, err))
	}
	defer func() {
		_ = f.Close()
	}()

	dir, _ := path.Split(file)
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Bytes()
		line = append(line, '\n')
		refPos := bytes.Index(line, refKey)
		if refPos == -1 {
			cache[file] = append(cache[file], line...)
			continue
		}
		if hashPos := bytes.Index(line, hash) hashPos != -1 {
			p := bytes.SplitN(line, hash)
			if len(p) == 2 {
				panic(fmt.Errorf("hash reference detected, replace by schema file: %q", string(line)))
			}

			panic(fmt.Errorf("hash reference detected, replace by schema file: %q", string(line)))
		}
		ref := bytes.Trim(bytes.TrimSpace(line[refPos+5:]),`"'`)

		refFile := path.Join(dir, string(ref))
		refData, ok := cache[refFile]
		if !ok {
			readFile(refFile, cache)
		}
		refData = cache[refFile]
		for _, refLine := range bytes.Split(refData, []byte("\n")) {
			cache[file] = append(cache[file], bytes.Repeat([]byte(" "), refPos)...)
			cache[file] = append(cache[file], refLine...)
		}
	}

	return nil
}
*/
