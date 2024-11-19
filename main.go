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
		Name:    "FAT32-SecRm",
		Usage:   "A Tool to remove file in FAT32 filesystem",
		Version: "0.0.1",

		Commands: []*cli.Command{
			{
				Name:    "remove",
				Aliases: []string{"r"},
				Usage:   "remove file or directory",
				Action: func(c *cli.Context) error {
					// 解析参数
					absFileName := c.Args().Get(0)
					switch runtime.GOOS {
					case "windows", "linux":
						return RemoveFile(absFileName)
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
