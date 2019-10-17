package client

import (
	"bufio"
	"context"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var headerRegex = regexp.MustCompile(`[^\s]+($|\s+)`)

func (c *k8sClient) ResourceTypes(ctx context.Context) (types []*APIResourceType, err error) {
	reader, writer := io.Pipe()
	go func() {
		args := []string{"api-resources", "--verbs", "delete"}
		e := kubectl(ctx, nil, writer, c.kubeconfigFile, args)
		writer.CloseWithError(e)
	}()
	types, err = parseResourceTypeTable(reader)
	reader.Close()
	return
}

func parseResourceTypeTable(reader io.Reader) (types []*APIResourceType, err error) {
	lineReader := bufio.NewReader(reader)
	colNames, readers, err := readTableHeader(lineReader)
	if err != nil {
		return
	}
	var name, shortName, apiGroup, kind, namespaced func(line string) string
	var colsFound int
	for i, colName := range colNames {
		switch colName {
		case "name":
			name = readers[i]
			colsFound |= 1
		case "shortnames":
			shortName = readers[i]
			colsFound |= 2
		case "apigroup":
			apiGroup = readers[i]
			colsFound |= 4
		case "kind":
			kind = readers[i]
			colsFound |= 8
		case "namespaced":
			namespaced = readers[i]
			colsFound |= 16
		}
	}
	if colsFound != 31 {
		return nil, errors.New("parse api-resources: missing NAME, SHORTNAMES, APIGROUP, KIND or NAMESPACED header column")
	}
	var rtype *APIResourceType
	var line string
	for {
		if line, err = lineReader.ReadString('\n'); err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		err = nil
		rtype = &APIResourceType{
			Name:     name(line),
			APIGroup: apiGroup(line),
			Kind:     kind(line),
		}
		if rtype.Name == "" {
			return nil, errors.Errorf("parse api-resources: empty NAME column")
		}
		if rtype.Kind == "" {
			return nil, errors.Errorf("parse api-resources: empty KIND column")
		}
		shortNameCsv := shortName(line)
		if shortNameCsv != "" {
			rtype.ShortNames = strings.Split(shortNameCsv, ",")
		}
		if rtype.Namespaced, err = strconv.ParseBool(namespaced(line)); err != nil {
			return nil, errors.Errorf("parse api-resources: namespaced column: %s", err)
		}
		types = append(types, rtype)
	}
}

func readTableHeader(reader *bufio.Reader) (colNames []string, colReaders []func(string) string, err error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	colNames = headerRegex.FindAllString(line, -1)
	colReaders = make([]func(string) string, len(colNames))
	var currPos int
	for i, col := range colNames {
		startPos := currPos
		if i != len(colNames)-1 {
			endPos := currPos + len(col)
			colReaders[i] = func(line string) string {
				if len(line) < endPos {
					return ""
				}
				return strings.TrimSpace(line[startPos:endPos])
			}
		} else {
			colReaders[i] = func(line string) string {
				if len(line) < startPos {
					return ""
				}
				return strings.TrimSpace(line[startPos:])
			}
		}
		colNames[i] = strings.ToLower(strings.TrimSpace(col))
		currPos += len(col)
	}
	return
}
