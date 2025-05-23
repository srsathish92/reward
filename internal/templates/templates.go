package templates

import (
	"bytes"
	"container/list"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/docker/cli/cli/compose/loader"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/rewardenv/reward/assets"
	"github.com/rewardenv/reward/internal/compose"
	"github.com/rewardenv/reward/pkg/util"
)

type Client struct {
	fs embed.FS
}

func New() *Client {
	return &Client{
		fs: assets.Assets,
	}
}

// Cwd returns the current working directory.
func (c *Client) Cwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		log.Panicln(err)
	}

	return cwd
}

func (c *Client) AppName() string {
	return viper.GetString("app_name")
}

// AppHomeDir returns the application's home directory.
func (c *Client) AppHomeDir() string {
	return viper.GetString(fmt.Sprintf("%s_home_dir", c.AppName()))
}

// ExecuteTemplate executes the templates, appending some specific template functions to the execution.
func (c *Client) ExecuteTemplate(t *template.Template, buffer io.Writer) error {
	data := viper.AllSettings()

	if err := t.Funcs(sprig.TxtFuncMap()).
		Funcs(funcMap()).
		ExecuteTemplate(buffer, t.Name(), data); err != nil {
		return errors.Wrapf(err, "cannot execute template %s", t.Name())
	}

	return nil
}

// AppendTemplatesFromPathsStatic appends templates to t from templateList list searching them in paths path list.
// This function looks up templates built to the application's binary (static files).
// If it cannot find templates it's not going to fail.
// If a template with the same name already exists, it's going to skip that template.
func (c *Client) AppendTemplatesFromPathsStatic(tpl *template.Template, templateList *list.List, paths []string) error {
	for _, path := range paths {
		// Use the regular paths without filepath.Join() because the assets are embedded with forward slashes.
		templatePath := strings.ReplaceAll(path, "\\", "/")

		searchT := tpl.Lookup(templatePath)
		if searchT != nil {
			continue
		}

		content, err := c.fs.ReadFile(templatePath)
		if err != nil {
			continue
		}

		child, err := template.New(templatePath).Funcs(funcMap()).Parse(string(content))
		if err != nil {
			return errors.Wrapf(err, "cannot parse template %s", path)
		}

		_, err = tpl.AddParseTree(child.Name(), child.Tree)
		if err != nil {
			return errors.Wrapf(err, "adding template %s to tree", child.Name())
		}

		templateList.PushBack(child.Name())
	}

	return nil
}

// AppendTemplatesFromPaths appends templates to t from templateList list searching them in paths path list.
// If it cannot find templates it's not going to fail.
// If a template with the same name already exists, it's going to skip that template.
func (c *Client) AppendTemplatesFromPaths(tpl *template.Template, templateList *list.List, paths []string) error {
	for _, path := range paths {
		// Make sure to convert the path slashes to Windows \ format if we're on Windows.
		filePathCwd := filepath.Join(c.Cwd(), fmt.Sprintf(".%s", c.AppName()), path)
		filePathAppHome := filepath.Join(c.AppHomeDir(), path)

		// First lookup templates in the current directory, then in the home directory.
		for directory, filePath := range map[string]string{"$CWD": filePathCwd, "app home": filePathAppHome} {
			if !util.FileExists(filePath) {
				log.Tracef("Template not found in %s: %s", directory, path)

				continue
			}

			searchT := tpl.Lookup(path)
			if searchT != nil {
				log.Tracef("Template already defined: %s. Skipping.", path)

				continue
			}

			child, err := template.New(path).Funcs(funcMap()).ParseFiles(filePath)
			if err != nil {
				return errors.Wrapf(err, "cannot parse template %s", path)
			}

			_, err = tpl.AddParseTree(child.Name(), child.Lookup(filepath.Base(filePath)).Tree)
			if err != nil {
				return errors.Wrapf(err, "adding template %s", child.Name())
			}

			templateList.PushBack(child.Name())
		}
	}

	return nil
}

// AppendEnvironmentTemplates tries to look up all the templates dedicated for an environment type.
func (c *Client) AppendEnvironmentTemplates(
	tpl *template.Template,
	templateList *list.List,
	partialName string,
	envType string,
) error {
	staticTemplatePaths := []string{
		filepath.Join(
			"templates",
			"docker-compose",
			"environments",
			"includes",
			fmt.Sprintf("%s.base.yml", partialName)),
		filepath.Join(
			"templates",
			"docker-compose",
			"environments",
			"includes",
			fmt.Sprintf("%s.%s.yml", partialName, runtime.GOOS),
		),
		filepath.Join(
			"templates",
			"docker-compose",
			"environments",
			envType,
			fmt.Sprintf("%s.base.yml", partialName)),
		filepath.Join(
			"templates",
			"docker-compose",
			"environments",
			envType,
			fmt.Sprintf("%s.%s.yml", partialName, runtime.GOOS),
		),
	}
	templatePaths := []string{
		filepath.Join(
			"templates",
			"docker-compose",
			"environments",
			"includes",
			fmt.Sprintf("%s.base.yml", partialName),
		),
		filepath.Join(
			"templates",
			"docker-compose",
			"environments",
			"includes",
			fmt.Sprintf("%s.%s.yml", partialName, runtime.GOOS),
		),
		filepath.Join(
			"templates",
			"docker-compose",
			"environments",
			envType,
			fmt.Sprintf("%s.base.yml", partialName),
		),
		filepath.Join(
			"templates",
			"docker-compose",
			"environments",
			envType,
			fmt.Sprintf("%s.%s.yml", partialName, runtime.GOOS),
		),
	}

	// First read the templates from the current directory. If they exist we will use them. If the don't
	//   then we will append them from the static content.
	if err := c.AppendTemplatesFromPaths(tpl, templateList, templatePaths); err != nil {
		return errors.Wrap(err, "cannot append templates from local paths")
	}

	if err := c.AppendTemplatesFromPathsStatic(tpl, templateList, staticTemplatePaths); err != nil {
		return errors.Wrap(err, "cannot append static templates")
	}

	return nil
}

// AppendMutagenTemplates is going to add mutagen configuration templates.
func (c *Client) AppendMutagenTemplates(
	tpl *template.Template,
	templateList *list.List,
	partialName string,
	envType string,
) error {
	staticTemplatePaths := []string{
		filepath.Join("templates", "docker-compose", "environments",
			envType,
			fmt.Sprintf("%s.%s.yml", envType, partialName)),
		filepath.Join(
			"templates", "docker-compose", "environments", envType,
			fmt.Sprintf("%s.%s.%s.yml", envType, partialName, runtime.GOOS),
		),
	}

	for _, v := range staticTemplatePaths {
		content, err := assets.Assets.ReadFile(v)
		if err != nil {
			log.Traceln(err)

			continue
		}

		child, err := template.New(v).Funcs(funcMap()).Parse(string(content))
		if err != nil {
			return errors.Wrapf(err, "cannot parse template %s", v)
		}

		_, err = tpl.AddParseTree(child.Name(), child.Tree)
		if err != nil {
			return errors.Wrapf(err, "adding template %s", child.Name())
		}

		templateList.PushBack(child.Name())
	}

	return nil
}

// SvcBuildDockerComposeTemplate builds the templates which are used to invoke docker compose for the common services.
func (c *Client) RunCmdSvcBuildDockerComposeTemplate(t *template.Template, templateList *list.List) error {
	templatePaths := []string{
		"templates/docker-compose/common-services/docker-compose.yml",
	}

	if err := c.AppendTemplatesFromPathsStatic(t, templateList, templatePaths); err != nil {
		return errors.Wrap(err, "cannot append common-services/docker-compose.yml static template")
	}

	return nil
}

// ConvertTemplateToComposeConfig iterates through all the templates and converts them to docker compose configurations.
func (c *Client) ConvertTemplateToComposeConfig(
	t *template.Template,
	templateList *list.List,
) (compose.ConfigDetails, error) {
	log.Debugln("Converting templates to docker compose configurations...")

	var (
		configs     = new(compose.ConfigDetails)
		configFiles = new([]compose.ConfigFile)
		bs          bytes.Buffer
	)

	for e := templateList.Front(); e != nil; e = e.Next() {
		bs.Reset()
		tplName := fmt.Sprint(e.Value)

		if err := c.ExecuteTemplate(t.Lookup(tplName), &bs); err != nil {
			return *configs, errors.Wrapf(err, "failed to execute template %s", tplName)
		}

		templateBytes := bs.Bytes()
		templateBytes = append(templateBytes, []byte("\n")...)

		composeConfig, err := loader.ParseYAML(templateBytes)
		if err != nil {
			return *configs, errors.Wrapf(err, "parsing template %s", tplName)
		}

		configFile := compose.ConfigFile{
			Filename: tplName,
			Config:   composeConfig,
		}

		*configFiles = append(*configFiles, configFile)
	}

	configs.ConfigFiles = *configFiles

	return *configs, nil
}

func funcMap() template.FuncMap {
	//nolint:varnamelen
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	extra := template.FuncMap{
		"include":  func(string, interface{}) string { return "not implemented" },
		"tpl":      func(string, interface{}) interface{} { return "not implemented" },
		"required": func(string, interface{}) (interface{}, error) { return "not implemented", nil },
		"lookup": func(string, string, string, string) (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		},
		"isEnabled": isEnabled,
		"parseKV":   ParseKV,
	}

	for k, v := range extra {
		f[k] = v
	}

	return f
}

func ParseKV(kvStr string) map[string]string {
	res := map[string]string{}

	kvPairRe := regexp.MustCompile(`(.*?)=([^=]*)(?:,|$)`)
	for _, kv := range kvPairRe.FindAllStringSubmatch(kvStr, -1) {
		res[kv[1]] = kv[2]
	}

	return res
}

// isEnabled returns true if given value is true (bool), 1 (int), "1" (string) or "true" (string).
func isEnabled(given interface{}) bool {
	g := reflect.ValueOf(given)
	if !g.IsValid() {
		return false
	}

	//nolint:exhaustive
	switch g.Kind() {
	case reflect.String:
		return strings.EqualFold(g.String(), "true") || g.String() == "1"
	case reflect.Bool:
		return g.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return g.Int() == 1
	default:
		return false
	}
}

// GenerateMutagenTemplateFile generates mutagen configuration from template if it doesn't exist.
func (c *Client) GenerateMutagenTemplateFile(path, envType string) error {
	if util.FileExists(path) {
		// Mutagen sync file already exists, skipping.
		return nil
	}

	var (
		bs                  bytes.Buffer
		mutagenTemplate     = new(template.Template)
		mutagenTemplateList = list.New()
	)

	if err := c.AppendMutagenTemplates(mutagenTemplate, mutagenTemplateList, "mutagen", envType); err != nil {
		return errors.Wrap(err, "appending mutagen templates")
	}

	for e := mutagenTemplateList.Front(); e != nil; e = e.Next() {
		tplName := fmt.Sprint(e.Value)

		if err := c.ExecuteTemplate(mutagenTemplate.Lookup(tplName), &bs); err != nil {
			return errors.Wrapf(err, "executing mutagen template")
		}
	}

	if err := util.CreateDirAndWriteToFile(bs.Bytes(), path, 0o640); err != nil {
		return errors.Wrapf(err, "cannot create mutagen sync file")
	}

	return nil
}

// SvcGenerateTraefikConfig generates the default traefik configuration.
func (c *Client) SvcGenerateTraefikConfig() error {
	var (
		bs      bytes.Buffer
		tpl     = template.New("traefik")
		tplList = list.New()
	)

	if err := c.AppendTemplatesFromPathsStatic(
		tpl,
		tplList,
		[]string{"templates/traefik/traefik.yml"},
	); err != nil {
		return errors.Wrapf(err, "cannot append traefik.yml template")
	}

	for e := tplList.Front(); e != nil; e = e.Next() {
		tplName := fmt.Sprint(e.Value)

		if err := c.ExecuteTemplate(tpl.Lookup(tplName), &bs); err != nil {
			return errors.Wrapf(err, "cannot execute traefik template %s", tplName)
		}
	}

	if err := util.CreateDirAndWriteToFile(
		bs.Bytes(),
		filepath.Join(c.AppHomeDir(), "etc/traefik/traefik.yml"),
		0o644,
	); err != nil {
		return errors.Wrapf(err, "cannot write traefik template file")
	}

	return nil
}

// SvcGenerateTraefikDynamicConfig generates the dynamic traefik configuration.
func (c *Client) SvcGenerateTraefikDynamicConfig(svcDomain string) error {
	traefikConfig := fmt.Sprintf(
		`tls:
  stores:
    default:
    defaultCertificate:
      certFile: /etc/ssl/certs/%[1]v.crt.pem
      keyFile: /etc/ssl/certs/%[1]v.key.pem
  certificates:`, svcDomain,
	)

	files, err := filepath.Glob(filepath.Join(c.AppHomeDir(), "ssl/certs", "*.crt.pem"))
	if err != nil {
		return errors.Wrapf(err, "cannot list ssl certificates")
	}

	log.Debugf("Available certificates: %s", files)

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".crt.pem")

		log.Tracef("Certificate file name: %s", name)
		log.Tracef("Certificate domain: %s", filepath.Ext(name))

		traefikConfig += fmt.Sprintf(
			`
    - certFile: /etc/ssl/certs/%[1]v.crt.pem
      keyFile: /etc/ssl/certs/%[1]v.key.pem
`, name,
		)
	}

	if err := util.CreateDirAndWriteToFile(
		[]byte(traefikConfig), filepath.Join(c.AppHomeDir(), "etc/traefik", "dynamic.yml"), 0o644,
	); err != nil {
		return errors.Wrap(err, "cannot write traefik dynamic configuration file")
	}

	return nil
}
