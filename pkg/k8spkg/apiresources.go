package k8spkg

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// APIResourceType represents a Kubernetes API resource type's metadata
type APIResourceType struct {
	Name       string
	ShortNames []string
	APIGroup   string
	Kind       string
	Namespaced bool
}

// Returns the type's short name if any or its name
func (t *APIResourceType) ShortName() (name string) {
	name = t.Name
	if len(t.ShortNames) > 0 {
		name = t.ShortNames[0]
	}
	return
}

// Returns the type's short name with APIGroup suffix if there is one
func (t *APIResourceType) FullName() (name string) {
	if t.APIGroup == "" {
		return t.ShortName()
	}
	return t.ShortName() + "." + t.APIGroup
}

var headerRegex = regexp.MustCompile(`[^\s]+($|\s+)`)

func LoadAPIResourceTypes(ctx context.Context, kubeconfigFile string) (types []*APIResourceType, err error) {
	var stdout, stderr bytes.Buffer
	args := []string{"api-resources", "--verbs", "delete"}
	if kubeconfigFile != "" {
		args = append([]string{"--kubeconfig", kubeconfigFile}, args...)
	}
	c := exec.CommandContext(ctx, "kubectl", args...)
	c.Stdout = &stdout
	c.Stderr = &stderr
	logrus.Debugf("Running %+v", c.Args)
	if err = c.Run(); err != nil {
		err = errors.Errorf("%+v: %s. %s", c.Args, err, strings.TrimSuffix(stderr.String(), "\n"))
	} else {
		types, err = parseApiResourceTable(bytes.NewReader(stdout.Bytes()))
	}
	return
}

func parseApiResourceTable(reader io.Reader) (types []*APIResourceType, err error) {
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
		return nil, errors.New("api-resources parser: missing NAME, SHORTNAMES, APIGROUP, KIND or NAMESPACED header column")
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
			return nil, errors.Errorf("api-resources parser: empty NAME column")
		}
		if rtype.Kind == "" {
			return nil, errors.Errorf("api-resources parser: empty KIND column")
		}
		shortNameCsv := shortName(line)
		if shortNameCsv != "" {
			rtype.ShortNames = strings.Split(shortNameCsv, ",")
		}
		if rtype.Namespaced, err = strconv.ParseBool(namespaced(line)); err != nil {
			return nil, errors.Errorf("api-resources parser: namespaced column: %s", err)
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
