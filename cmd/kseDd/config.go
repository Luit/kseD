package main

import (
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"io"
	"sync"
)

var (
	confmu  sync.Mutex // Config lock
	heights [][2]uint16
	uid4    map[[4]byte]int
	uid7    map[[7]byte]int
	uid10   map[[10]byte]int
)

func bytes4(b []byte) [4]byte {
	return [4]byte{
		b[0],
		b[1],
		b[2],
		b[3],
	}
}

func bytes7(b []byte) [7]byte {
	return [7]byte{
		b[0],
		b[1],
		b[2],
		b[3],
		b[4],
		b[5],
		b[6],
	}
}

func bytes10(b []byte) [10]byte {
	return [10]byte{
		b[0],
		b[1],
		b[2],
		b[3],
		b[4],
		b[5],
		b[6],
		b[7],
		b[8],
		b[9],
	}
}

type config struct {
	Heights [2]uint16 `json:"heights"`
	Cards   [][]byte  `json:"cards"`
}

func readConfig(r io.Reader) error {
	d := json.NewDecoder(r)
	var v []config
	if err := d.Decode(&v); err != nil {
		return err
	}
	newHeights := make([][2]uint16, len(v))
	newUid4 := make(map[[4]byte]int)
	newUid7 := make(map[[7]byte]int)
	newUid10 := make(map[[10]byte]int)
	for i, u := range v {
		newHeights[i] = u.Heights
		for _, c := range u.Cards {
			switch len(c) {
			case 4:
				newUid4[bytes4(c)] = i
			case 7:
				newUid7[bytes7(c)] = i
			case 10:
				newUid10[bytes10(c)] = i
			default:
				return errors.New("unexpected card length")
			}
		}
	}
	confmu.Lock()
	heights = newHeights
	uid4 = newUid4
	uid7 = newUid7
	uid10 = newUid10
	confmu.Unlock()
	return nil
}

func loadConfig(currentSha string) (string, error) {
	commitSha, err := currentMaster()
	if err != nil {
		return "", errors.New("failed to find master branch: " + err.Error())
	}
	if commitSha == currentSha {
		return commitSha, nil
	}
	treeSha, err := getTree(commitSha)
	if err != nil {
		return "", errors.New("failed to find tree: %s" + err.Error())
	}
	blobSha, err := getBlob(treeSha, "kseD.json")
	if err != nil {
		return "", errors.New("failed to find blob: %s" + err.Error())
	}
	r, err := blobReader(blobSha)
	if err != nil {
		return "", errors.New("failed to read blob: %s" + err.Error())
	}
	return commitSha, readConfig(r)
}

func init() {
	expvar.Publish("config", expvar.Func(func() interface{} {
		confmu.Lock()
		defer confmu.Unlock()
		cards := make(map[string]int)
		for k, v := range uid4 {
			key := fmt.Sprintf("%02x%02x%02x%02x",
				k[0],
				k[1],
				k[2],
				k[3],
			)
			cards[key] = v
		}
		for k, v := range uid7 {
			key := fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x",
				k[0],
				k[1],
				k[2],
				k[3],
				k[4],
				k[5],
				k[6],
			)
			cards[key] = v
		}
		for k, v := range uid10 {
			key := fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x",
				k[0],
				k[1],
				k[2],
				k[3],
				k[4],
				k[5],
				k[6],
				k[7],
				k[8],
				k[9],
			)
			cards[key] = v
		}
		return struct {
			Heights [][2]uint16    `json:"heights"`
			Cards   map[string]int `json:"cards"`
		}{
			Heights: heights,
			Cards:   cards,
		}
	}))
}
