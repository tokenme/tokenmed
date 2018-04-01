package redpacket

import (
	//"github.com/davecgh/go-spew/spew"
	"github.com/tokenme/tokenmed/utils"
)

func Generate(total uint64, num uint64, min uint64) []uint64 {
	var (
		i    uint64 = 1
		resp []uint64
	)
	for i < num {
		safeTotal := (total - (num-i)*min) / (num - i) //随机安全上限
		money := utils.RangeRandUint64(min, safeTotal)
		resp = append(resp, money)
		total = total - money
		i += 1
	}
	resp = append(resp, total)
	//spew.Dump(resp)
	return resp
}
