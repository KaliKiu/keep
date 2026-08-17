package main

func (l Letter) InboxLetterSort(username string) bool {
	return (!l.IsSender && !l.IsRead) || (l.LatestReplyUsername != "" && l.LatestReplyUsername != username && !l.LatestReplyRead)
}
func (l Letter) HistoryRxSort(username string) bool {
	return !l.IsSender && l.IsRead && !l.InboxLetterSort(username)
}
func (l Letter) HistoryTxSort(username string) bool {
	return l.IsSender && !l.InboxLetterSort(username)
}
func (u User) HasUnreadInbox(letters []Letter) bool {
	for _, l := range letters {
		if l.InboxLetterSort(u.Username) {
			return true
		}
	}
	return false
}
