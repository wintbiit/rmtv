package bilibili

import (
	"regexp"
	"testing"
)

func TestPostProcess(t *testing.T) {
	re := regexp.MustCompile(`<em[^>]*>(.*?)</em>`)
	const test = "【熬夜冠军赛】定了3晚酒店连床都没见到<em class=\"keyword\">机甲大师</em>高校人工智能挑战赛。我如何匹配这串字符串中的em 标签a"

	result := re.ReplaceAllString(test, `**$1**`)
	t.Log(result)
}
