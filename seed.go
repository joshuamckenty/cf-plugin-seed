package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/cloudfoundry/cli/plugin"
	"github.com/codegangsta/cli"
	"gopkg.in/yaml.v2"
)

//VERSION of seeder
const VERSION = "0.0.1"

func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stdout, "error:", err)
		os.Exit(1)
	}
}

func main() {
	plugin.Start(&SeedPlugin{})
}

//SeedPlugin empty struct for plugin
type SeedPlugin struct{}

//Run of seeder plugin
func (plugin SeedPlugin) Run(cliConnection plugin.CliConnection, args []string) {
	app := cli.NewApp()
	app.Name = "seed"
	app.Version = VERSION
	app.Author = "Long Nguyen"
	app.Email = "long.nguyen11288@gmail.com"
	app.Usage = "Seeds Cloud Foundry and setups apps/orgs/services on a given Cloud Foundry setup"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "f",
			Value: "",
			Usage: "seed manifest for seeding Cloud Foundry",
		},
	}
	app.Action = func(c *cli.Context) {
		fileName := c.String("f")
		seedRepo := NewSeedRepo(cliConnection, fileName)

		err := seedRepo.ReadManifest()
		fatalIf(err)

		err = seedRepo.CreateOrganizations()
		fatalIf(err)

		err = seedRepo.CreateSpaces()
		fatalIf(err)

		err = seedRepo.CreateApps()
		fatalIf(err)
	}
	app.Run(args)
}

//GetMetadata of plugin
func (SeedPlugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "seed",
		Commands: []plugin.Command{
			{
				Name:     "seed",
				HelpText: "Seeds Cloud Foundry and setups apps/orgs/services on new Cloud Foundry setup",
			},
		},
	}
}

//SeedRepo of cli
type SeedRepo struct {
	conn     plugin.CliConnection
	fileName string
	Manifest SeederManifest
}

func NewSeedRepo(conn plugin.CliConnection, fileName string) *SeedRepo {
	return &SeedRepo{
		conn:     conn,
		fileName: fileName,
	}
}

func (repo *SeedRepo) ReadManifest() error {
	file, err := ioutil.ReadFile(repo.fileName)
	if err != nil {
		return err
	}
	repo.Manifest = SeederManifest{}

	err = yaml.Unmarshal(file, &repo.Manifest)
	if err != nil {
		return err
	}

	return nil
}

func (repo *SeedRepo) CreateOrganizations() error {
	for _, org := range repo.Manifest.Organizations {
		_, err := repo.conn.CliCommand("create-org", org.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (repo *SeedRepo) CreateSpaces() error {
	for _, org := range repo.Manifest.Organizations {
		repo.conn.CliCommand("target", "-o", org.Name)
		for _, space := range org.Spaces {
			_, err := repo.conn.CliCommand("create-space", space.Name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (repo *SeedRepo) CreateApps() error {
	for _, org := range repo.Manifest.Organizations {
		for _, space := range org.Spaces {
			repo.conn.CliCommand("target", "-o", org.Name, "-s", space.Name)
			for _, app := range space.Apps {
				err := repo.DeployApp(app)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

//DeployApp deploys a single app
func (repo *SeedRepo) DeployApp(app App) error {
	args := []string{"push", app.Name}
	if app.Repo != "" {
		wd, _ := os.Getwd()
		appPath := wd + "/apps/" + app.Name
		os.MkdirAll(appPath, 0777)

		files, _ := ioutil.ReadDir(appPath)

		if len(files) == 0 {
			gitPath, err := exec.LookPath("git")
			if err != nil {
				return err
			}
			err = exec.Command(gitPath, "clone", app.Repo, appPath).Run()
			if err != nil {
				return nil
			}
		}
		args = append(args, "-p", appPath)

	} else if app.Path != "" {
		args = append(args, "-p", app.Path)
	} else {
		errMsg := fmt.Sprintf("App need repo or path %s", app.Name)
		return errors.New(errMsg)
	}

	if app.Disk != "" {
		args = append(args, "-k", app.Disk)
	}
	if app.Memory != "" {
		args = append(args, "-m", app.Memory)
	}
	if app.Instances != "" {
		args = append(args, "-i", app.Instances)
	}
	if app.Hostname != "" {
		args = append(args, "-n", app.Hostname)
	}
	if app.Domain != "" {
		args = append(args, "-d", app.Domain)
	}
	if app.Buildpack != "" {
		args = append(args, "-b", app.Buildpack)
	}

	repo.conn.CliCommand(args...)

	return nil
}
