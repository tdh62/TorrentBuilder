package main

import (
	"crypto"
	_ "crypto/sha1"
	"fmt"
	"github.com/zeebo/bencode"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	targetpath := ""
	if len(os.Args) < 2 {
		fmt.Println("Usage: TorrentBuilder <target path>")
	} else {
		targetpath = os.Args[1]
	}

	// check if the target path is valid and check if the target path is a directory
	fileInfo, err := os.Stat(targetpath)
	if os.IsNotExist(err) {
		panic(err)
	}
	log.Println("开始计算hash...")

	fileList := []FileList{}

	sha1 := []byte{}
	stop := make(chan struct{})
	calcdone := make(chan struct{})
	files := make(chan string)
	go calcHash(files, &sha1, stop, calcdone)

	if fileInfo.IsDir() {
		//	Multi file torrent
		err = filepath.Walk(targetpath, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				paths, _ := filepath.Rel(targetpath, path)
				if strings.Contains(paths, "\\") {
					paths = strings.ReplaceAll(paths, "\\", "/")
				}
				fileList = append(fileList, FileList{
					Length: int(info.Size()),
					Path:   strings.Split(paths, "/"),
				})
				//	calc the sha1
				files <- path
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
	} else {
		//	Single file torrent
		fileList = append(fileList, FileList{
			Length: int(fileInfo.Size()),
			Path:   []string{targetpath},
		})
		//	calc the sha1
		files <- targetpath
	}

	close(stop)
	<-calcdone
	log.Println("计算hash完成，生成torrent文件...")
	encoded := []byte{}
	if !fileInfo.IsDir() {
		//	Single file torrent
		torrent := SingleFileTorrentStruct{
			Announce:     "https://www.pttime.org/announce.php",
			Createdby:    "tdhTorrentBuilder v0.1",
			Creationdate: int(time.Now().Unix()),
			Info: SingleFileTorrentInfo{
				Name:        fileInfo.Name(),
				PieceLength: 32 * 1024 * 1024,
				Pieces:      sha1,
				Length:      int(fileInfo.Size()),
			},
		}
		encoded, err = bencode.EncodeBytes(torrent)
		if err != nil {
			return
		}
	} else {
		//	Multi file torrent
		torrent := MultiFileTorrentStruct{
			Announce:     "https://www.pttime.org/announce.php",
			Createdby:    "tdhTorrentBuilder v0.1",
			Creationdate: int(time.Now().Unix()),
			Info: MultiFileTorrentInfo{
				Name:        fileInfo.Name(),
				PieceLength: 32 * 1024 * 1024,
				Pieces:      sha1,
				Files:       fileList,
			},
		}
		// save to torrent file
		encoded, err = bencode.EncodeBytes(torrent)
		if err != nil {
			return
		}
	}
	_, targetpathfilename := filepath.Split(targetpath)
	fmt.Println(targetpathfilename)
	torrentfile, err := os.Create(targetpathfilename + ".torrent")
	if err != nil {
		panic(err)
	}
	defer torrentfile.Close()
	torrentfile.Write(encoded)
}

func calcHash(files chan string, sha1 *[]byte, stop chan struct{}, calcdone chan struct{}) {
	blockSize := 32 * 1024 * 1024
	cn := 0
	h := crypto.SHA1.New()
	for {
		select {
		case <-stop:
			*sha1 = append(*sha1, h.Sum(nil)...)
			fmt.Println("calc hash done")
			calcdone <- struct{}{}
			return
		case fpath := <-files:
			log.Println("处理文件:", fpath)
			f, err := os.Open(fpath)
			if err != nil {
				panic(err)
			}
			defer f.Close()
			for {
				hashbuf := make([]byte, blockSize-cn)
				n, err := f.Read(hashbuf)
				if err != nil {
					if err.Error() == "EOF" {
						break
					} else {
						panic(err)
					}
				}
				if n == 0 {
					break
				}
				cn += n
				h.Write(hashbuf[:n])
				//fmt.Println(cn, n)
				if cn == blockSize {
					*sha1 = append(*sha1, h.Sum(nil)...)
					cn = 0
					h.Reset()
				}
			}
		}
	}
}

type SingleFileTorrentStruct struct {
	Announce     string                `bencode:"announce"`
	Createdby    string                `bencode:"createdby"`
	Creationdate int                   `bencode:"creationdate"`
	Info         SingleFileTorrentInfo `bencode:"info"`
}

type MultiFileTorrentStruct struct {
	Announce     string               `bencode:"announce"`
	Createdby    string               `bencode:"createdby"`
	Creationdate int                  `bencode:"creationdate"`
	Info         MultiFileTorrentInfo `bencode:"info"`
}

type SingleFileTorrentInfo struct {
	Name        string `bencode:"name"`
	PieceLength int    `bencode:"piece length"`
	Pieces      []byte `bencode:"pieces"`
	Length      int    `bencode:"length"`
}

type FileList struct {
	Length int      `bencode:"length"`
	Path   []string `bencode:"path"`
}

type MultiFileTorrentInfo struct {
	Name        string     `bencode:"name"`
	PieceLength int        `bencode:"piece length"`
	Pieces      []byte     `bencode:"pieces"`
	Files       []FileList `bencode:"files"`
}
