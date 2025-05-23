package logic

import (
	"bytes"
	"container/list"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/rewardenv/reward/internal/templates"
	"github.com/rewardenv/reward/pkg/util"
)

// bootstrap runs a full WordPress bootstrap process.
func (c *wordpress) bootstrap() error {
	if !util.AskForConfirmation("Would you like to bootstrap Wordpress?") {
		return nil
	}

	log.Println("Bootstrapping Wordpress...")

	if err := c.prepare(); err != nil {
		return errors.Wrap(err, "preparing bootstrap")
	}

	if _, err := c.download(); err != nil {
		return errors.Wrap(err, "downloading wordpress")
	}

	if err := c.installWPConfig(); err != nil {
		return errors.Wrap(err, "configuring wordpress")
	}

	log.Printf("Base Url: https://%s", c.TraefikFullDomain())
	log.Println("...bootstrap process finished.")

	return nil
}

func (c *wordpress) download() (downloaded bool, err error) {
	if c.SkipComposerInstall() {
		return false, nil
	}

	if util.FileExists(filepath.Join(c.Cwd(), c.WebRoot(), "index.php")) {
		return false, nil
	}

	log.Println("Downloading and installing WordPress...")

	command := "wget -qO /tmp/wordpress.tar.gz https://wordpress.org/latest.tar.gz"
	if err := c.RunCmdEnvExec(command); err != nil {
		return false, errors.Wrap(err, "downloading wordpress")
	}

	command = "tar -zxf /tmp/wordpress.tar.gz --strip-components=1 -C /var/www/html"
	if err := c.RunCmdEnvExec(command); err != nil {
		return false, errors.Wrap(err, "extracting wordpress")
	}

	command = "rm -f /tmp/wordpress.tar.gz"
	if err := c.RunCmdEnvExec(command); err != nil {
		return false, errors.Wrap(err, "removing wordpress archive")
	}

	log.Println("...WordPress downloaded.")

	return true, nil
}

func (c *wordpress) installWPConfig() error {
	var (
		bs             bytes.Buffer
		configFilePath = filepath.Join(c.Cwd(), c.WebRoot(), "wp-config.php")
		tpl            = new(template.Template)
		tmpList        = new(list.List)
		tplPath        = []string{
			filepath.Join("templates", "wordpress", "wp-config.php"),
		}
	)

	if util.CheckFileExistsAndRecreate(configFilePath) {
		return nil
	}

	if err := templates.New().AppendTemplatesFromPathsStatic(tpl, tmpList, tplPath); err != nil {
		return errors.Wrap(err, "loading wordpress wp-config.php template")
	}

	if c.DBPrefix() != "" {
		c.Set("wordpress_table_prefix", c.DBPrefix())
	}

	for e := tmpList.Front(); e != nil; e = e.Next() {
		tplName := fmt.Sprint(e.Value)

		if err := templates.New().ExecuteTemplate(tpl.Lookup(tplName), &bs); err != nil {
			return errors.Wrap(err, "executing wordpress wp-config.php template")
		}

		if err := util.CreateDirAndWriteToFile(bs.Bytes(), configFilePath); err != nil {
			return errors.Wrap(err, "writing wordpress wp-config.php file")
		}
	}

	return nil
}
