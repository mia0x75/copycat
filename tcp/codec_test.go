package tcp

import (
	"bytes"
	"fmt"
	"testing"
)

func TestCodec_Encode(t *testing.T) {
	msgID := int64(1)
	data := []byte("hello")

	codec := &Codec{}
	cc := codec.Encode(msgID, data)

	mid, c, p, err := codec.Decode(cc)
	fmt.Println(mid, c, p, err)
	if err != nil {
		t.Errorf(err.Error())
	}
	if mid != msgID {
		t.Error("error")
	}

	if !bytes.Equal(c, data) {
		t.Error("error 2")
	}

	content := make([]byte, 0)
	content = append(content, []byte("你好")...)
	content = append(content, cc...)
	content = append(content, []byte("你好")...)
	content = append(content, cc...)
	content = append(content, []byte("qwrqwerfq34wfq")...)

	mid, c, p, err = codec.Decode(content)
	fmt.Println(mid, c, p, err)
	if err != nil {
		t.Errorf(err.Error())
	}
	if mid != msgID {
		t.Error("error")
	}

	if !bytes.Equal(c, data) {
		t.Error("error 2")
	}

	content = append(content[:0], content[p:]...)

	mid, c, p, err = codec.Decode(content)
	if err != nil {
		t.Errorf(err.Error())
	}
	if mid != msgID {
		t.Error("error")
	}

	if !bytes.Equal(c, data) {
		t.Error("error 2")
	}

	content = append(content[:0], content[p:]...)

	mid, c, p, err = codec.Decode(content)
	fmt.Println(mid, c, p, err)
	if err == nil {
		t.Errorf("error")
	}
	if mid == msgID {
		t.Error("error")
	}

	if bytes.Equal(c, data) {
		t.Error("error 2")
	}

}
