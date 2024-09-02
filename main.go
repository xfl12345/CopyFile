package main

import (
	"bytes"
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type CopyFileConfig struct {
	Src  string `yaml:"source"`
	Dest string `yaml:"destination"`
}

type RootConfig struct {
	CopyFile []CopyFileConfig `yaml:"copy-file"`
}

func HandleError(err error) {
	if err != nil {
		log.Fatal(err.Error())
	}
}

func CopyFile(src, dst string) error {
	// 打开源文件
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("could not open source file: %w", err)
	}
	defer func(sourceFile *os.File) {
		err := sourceFile.Close()
		if err != nil {
			HandleError(err)
		}
	}(sourceFile)

	// 创建目标文件
	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("could not create destination file: %w", err)
	}
	defer func(destinationFile *os.File) {
		err := destinationFile.Close()
		if err != nil {
			HandleError(err)
		}
	}(destinationFile)

	// 将源文件内容复制到目标文件
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("could not copy file: %w", err)
	}

	// 确保写入磁盘
	err = destinationFile.Sync()
	if err != nil {
		return fmt.Errorf("could not flush to disk: %w", err)
	}

	return nil
}

// CopyDir 递归复制目录
func CopyDir(src, dst string) error {
	var err error
	var fds []os.DirEntry

	// 创建目标目录
	err = os.MkdirAll(dst, 0755)
	if err != nil {
		return err
	}

	// 读取源目录中的条目
	fds, err = os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, fd := range fds {
		srcPath := filepath.Join(src, fd.Name())
		dstPath := filepath.Join(dst, fd.Name())

		fileInfo, err := fd.Info()
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			// 如果是目录，则递归复制
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// 如果是文件，则复制文件
			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func PathResolve(thePath string) string {
	theExpandedPath := os.Expand(thePath, func(s string) string {
		return os.Getenv(s)
	})
	result, err := filepath.Abs(theExpandedPath)
	if err != nil {
		HandleError(err)
	}

	return result
}

func main() {
	theDefaultConfigFilePath := "./copy-file.yml"
	runInConsole := true
	//定义命令行参数方式1
	var originConfigFilePath string
	//flag.StringVar(&originConfigFilePath, "config", "./copy-file.yml", "The configuration file path.")
	//flag.StringVar(&originConfigFilePath, "c", "./copy-file.yml", "The configuration file path. (same as --config)")
	flag.StringVar(&originConfigFilePath, "c", "", "The configuration file path. Default to ["+theDefaultConfigFilePath+"].")
	//解析命令行参数
	flag.Parse()

	noFlagArgs := flag.Args()

	if originConfigFilePath == "" {
		if len(noFlagArgs) > 0 {
			originConfigFilePath = noFlagArgs[0]
		} else {
			originConfigFilePath = "./copy-file.yml"
		}

		runInConsole = false
	}

	originConfigFilePath = PathResolve(originConfigFilePath)
	configFileInfo, err := os.Stat(originConfigFilePath)
	if os.IsNotExist(err) {
		log.Fatal("The configuration file does not exist.")
	} else if configFileInfo.IsDir() {
		log.Fatal("The configuration file can not be a directory!")
	}

	if !runInConsole {
		originConfigFileParentDir := filepath.Dir(originConfigFilePath)
		err = os.Chdir(originConfigFileParentDir)
		HandleError(err)
	}

	//fmt.Println("Hello, world!")
	//// 获取单个环境变量
	//value := os.Getenv("PATH") // "PATH"为环境变量名称
	//fmt.Println(value)         // 输出环境变量值
	//homePath := os.Getenv("HOMEPATH")
	//log.Println("Current the value of ENV VAR [HOMEPATH] is [" + homePath + "]")
	//targetFilePath := filepath.Join(homePath, ".npmrc")
	//fmt.Println(targetFilePath)

	log.SetPrefix("[CopyFile] ")

	separator := string(filepath.Separator)
	log.Println("Current file path separator is [" + separator + "]")
	homeDir, err := os.UserHomeDir()
	HandleError(err)
	log.Println("Current user home directory path is [" + homeDir + "]")
	//pwd, err := os.Getwd()
	pwd, err := filepath.Abs(".")
	log.Println("Current work directory path is [" + pwd + "]")
	configFilePath := os.Expand(originConfigFilePath, func(s string) string {
		return os.Getenv(s)
	})
	configFilePath, err = filepath.Abs(configFilePath)
	yamlFileBytes, err := os.ReadFile(configFilePath)
	HandleError(err)
	log.Println("Current configuration file path is [" + configFilePath + "]")

	rootConf := new(RootConfig)
	err = yaml.NewDecoder(bytes.NewReader(yamlFileBytes)).Decode(&rootConf)
	HandleError(err)

	theTaskList := rootConf.CopyFile
	for i := 0; i < len(theTaskList); i++ {
		theRawSrc := theTaskList[i].Src
		src := PathResolve(theRawSrc)
		theRawDest := theTaskList[i].Dest
		dest := PathResolve(theRawDest)
		log.Printf("Task %d: Copy file from [%s] to [%s].\n", i+1, src, dest)
		srcInfo, err := os.Stat(src)
		if os.IsNotExist(err) {
			log.Println("Source file does not exist. Skip.")
			continue
		}

		if srcInfo.IsDir() {
			// 无论目标路径怎么写，都强制认为是一个目录
			destInfo, err := os.Stat(dest)
			if os.IsNotExist(err) {
				err = os.MkdirAll(dest, 0755)
				HandleError(err)
			} else if !destInfo.IsDir() {
				log.Fatal("Destination exists but it is not a directory")
			}

			err = CopyDir(src, dest)
			HandleError(err)
		} else {
			_, err := os.Stat(dest)
			if os.IsNotExist(err) {
				err = os.MkdirAll(dest, 0755)
				HandleError(err)
			}
			_, srcFile := filepath.Split(src)
			err = CopyFile(src, filepath.Join(dest, srcFile))
		}
	}

	log.Println("All done!")
	if !runInConsole {
		countDown := 5
		log.Println("The program will exit in " + strconv.Itoa(countDown) + " seconds. You have enough time to take screenshots of the console output.")
		for i := 0; i < countDown; i++ {
			fmt.Print(strconv.Itoa(countDown-i) + "...")
			time.Sleep(time.Second)
		}
		fmt.Println("Exit.")
	}
}
