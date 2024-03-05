package controller

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
	lObjects "github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/ingress/consts"
	apiv1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
)

func EncodeToValidName(str string) string {
	// Only letters (a-z, A-Z, 0-9, '_', '.', '-') are allowed.
	// the other char will repaced by ":{number}:"
	for _, char := range str {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '_' || char == '.' || char == '-' {
			continue
		}
		str = strings.ReplaceAll(str, string(char), fmt.Sprintf("-%d-", char))
	}
	return str
}
func DecodeFromValidName(str string) string {
	r, _ := regexp.Compile("-[0-9]+-")
	matchs := r.FindStringSubmatch(str)
	for _, match := range matchs {
		number, _ := strconv.Atoi(match[1 : len(match)-1])
		str = strings.ReplaceAll(str, match, fmt.Sprintf("%c", number))
	}
	return str
}

// hash a string to a string have 10 char
func HashString(str string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(str)))
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func TrimString(str string, length int) string {
	return str[:MinInt(len(str), length)]
}

// NodeNames get all the node names.
func NodeNames(nodes []*apiv1.Node) []string {
	ret := make([]string, len(nodes))
	for i, node := range nodes {
		ret[i] = node.Name
	}
	return ret
}

// NodeSlicesEqual check if two nodes equals to each other.
func NodeSlicesEqual(x, y []*apiv1.Node) bool {
	if len(x) != len(y) {
		return false
	}
	return stringSlicesEqual(NodeNames(x), NodeNames(y))
}

func stringSlicesEqual(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	if !sort.StringsAreSorted(x) {
		sort.Strings(x)
	}
	if !sort.StringsAreSorted(y) {
		sort.Strings(y)
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func GetResourceHashName(ing *nwv1.Ingress, clusterID string) string {
	fullName := fmt.Sprintf("%s_%s_%s", clusterID, ing.Namespace, ing.Name)
	hash := HashString(fullName)
	return hash
}

// GetResourceName get Ingress related resource name.
func GetResourceName(ing *nwv1.Ingress, clusterID string) string {
	hash := GetResourceHashName(ing, clusterID)
	return fmt.Sprintf("vks_%s_%s_%s_%s", clusterID[8:16], TrimString(ing.Namespace, 10), TrimString(ing.Name, 10), TrimString(hash, consts.DEFAULT_HASH_NAME_LENGTH))
}
func GetPolicyName(prefix string, mode bool, ruleIndex, pathIndex int) string {
	return fmt.Sprintf("%s_%t_r%d_p%d", prefix, mode, ruleIndex, pathIndex)
}
func GetPoolName(prefix, serviceName string, port int) string {
	return fmt.Sprintf("%s_%s_%d", prefix, TrimString(strings.ReplaceAll(serviceName, "/", "-"), 35), port)
}

func GetCertificateName(clusterID, namespace, name string) string {
	fullName := fmt.Sprintf("%s-%s-%s", clusterID, namespace, name)
	hashName := HashString(fullName)
	newName := fmt.Sprintf("vks-%s-%s-%s-%s-", clusterID[8:16], TrimString(namespace, 10), TrimString(name, 10), TrimString(hashName, consts.DEFAULT_HASH_NAME_LENGTH))
	for _, char := range newName {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '-' && char != '.' {
			newName = strings.ReplaceAll(newName, string(char), "-")
		}
	}
	return newName
}

func CheckIfPoolMemberExist(mems []*pool.Member, mem *pool.Member) bool {
	for _, r := range mems {
		if r.IpAddress == mem.IpAddress &&
			r.Port == mem.Port &&
			r.MonitorPort == mem.MonitorPort &&
			r.Backup == mem.Backup &&
			// r.Name == mem.Name &&
			r.Weight == mem.Weight {
			return true
		}
	}
	return false
}

func CheckIfPoolMemberExist2(mems []*lObjects.Member, mem *pool.Member) bool {
	for _, r := range mems {
		if r.Address == mem.IpAddress &&
			r.ProtocolPort == mem.Port &&
			r.MonitorPort == mem.MonitorPort &&
			r.Backup == mem.Backup &&
			// r.Name == mem.Name &&
			r.Weight == mem.Weight {
			return true
		}
	}
	return false
}

func ConvertObjectToPoolMember(obj *lObjects.Member) *pool.Member {
	return &pool.Member{
		IpAddress:   obj.Address,
		Port:        obj.ProtocolPort,
		MonitorPort: obj.MonitorPort,
		Backup:      obj.Backup,
		Weight:      obj.Weight,
		Name:        obj.Name,
	}
}

func ConvertObjectToPoolMemberArray(obj []*lObjects.Member) []*pool.Member {
	ret := make([]*pool.Member, len(obj))
	for i, m := range obj {
		ret[i] = ConvertObjectToPoolMember(m)
	}
	return ret
}

func ComparePoolMembers(p1, p2 []*pool.Member) bool {
	if len(p1) != len(p2) {
		return false
	}
	for _, m := range p2 {
		if !CheckIfPoolMemberExist(p1, m) {
			logrus.Infof("member in pool not exist: %v", m)
			return false
		}
	}
	return true
}
