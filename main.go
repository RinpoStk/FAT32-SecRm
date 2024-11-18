package main

import (
	"errors"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"runtime"
)

func main() {
	// 创建一个 CLI 应用
	app := &cli.App{
		Name:    "FAT32Del",
		Usage:   "a simple FAT32 file system delete tools",
		Version: "0.0.1",

		Commands: []*cli.Command{
			{
				Name:    "delete",
				Aliases: []string{"d"},
				Usage:   "delete file or directory",
				Action: func(c *cli.Context) error {
					// 解析参数
					absFileName := c.Args().Get(0)
					switch runtime.GOOS {
					case "windows", "linux":
						return DeleteFile(absFileName)
					default:
						return errors.New("not support right now")
					}
				},
			},
		},
	}

	// 运行 CLI 应用
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
