package agent

func (c *Clients) append(node *ClientNode) {
	*c = append(*c, node)
}

func (c *Clients) send(msgID int64, data []byte) {
	for _, node := range *c {
		node.Send(msgID, data)
	}
}

func (c *Clients) asyncSend(msgID int64, data []byte) {
	for _, node := range *c {
		node.AsyncSend(msgID, data)
	}
}

func (c *Clients) remove(node *ClientNode) {
	for index, n := range *c {
		if n == node {
			*c = append((*c)[:index], (*c)[index+1:]...)
			break
		}
	}
}

func (c *Clients) close() {
	for _, node := range *c {
		node.close()
	}
}
