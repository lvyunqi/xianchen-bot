package service

import (
	"fmt"
	"strings"
	"testing"

	"xianlv/internal/model"
)

func TestJournalMessagesAndNotificationInbox(t *testing.T) {
	game, store := testGame(t)
	writer := registerPlayer(t, game, "journal-writer", "松间客")
	receiver := registerPlayer(t, game, "journal-receiver", "照水真人")
	for index := 0; index < 7; index++ {
		entry := fmt.Sprintf("山门问心感悟%d：守一念清明，观灵泉周流。", index+1)
		result, handled, err := game.Execute("group", writer.AccountID, mustParse(t, "日记 "+entry))
		if err != nil || !handled || !strings.Contains(result.Title, "感悟已记") {
			t.Fatalf("write journal %d: handled=%v err=%v result=%+v", index, handled, err, result)
		}
	}
	journal, handled, err := game.Execute("group", writer.AccountID, mustParse(t, "日记"))
	if err != nil || !handled || !strings.Contains(journal.Content, "共7篇") || !containsAction(journal.Actions, "日记 2") || hasGlobalPagination(journal) {
		t.Fatalf("journal list: handled=%v err=%v result=%+v", handled, err, journal)
	}
	second, handled, err := game.Execute("group", writer.AccountID, mustParse(t, "日记 2"))
	if err != nil || !handled || !strings.Contains(second.Content, "第2/2页") {
		t.Fatalf("journal second page: handled=%v err=%v result=%+v", handled, err, second)
	}

	message, handled, err := game.Execute("group", writer.AccountID, mustParse(t, "留言\n@"+receiver.DaoName+"\n愿你破境顺遂，道心长明"))
	if err != nil || !handled || !strings.Contains(message.Title, "已送达") {
		t.Fatalf("newline message: handled=%v err=%v result=%+v", handled, err, message)
	}
	inbox, handled, err := game.Execute("group", receiver.AccountID, mustParse(t, "留言"))
	if err != nil || !handled || !strings.Contains(inbox.Content, "愿你破境顺遂") || !strings.Contains(inbox.Content, writer.DaoName) {
		t.Fatalf("received messages: handled=%v err=%v result=%+v", handled, err, inbox)
	}

	if _, _, err := game.Execute("group", writer.AccountID, mustParse(t, "留言 @"+receiver.DaoName+" 灵脉潮汐已至，可前往打坐")); err != nil {
		t.Fatal(err)
	}
	request := model.SocialMessage{SenderID: writer.ID, ReceiverID: receiver.ID, Type: "couple_request", Content: "请求结缘", Read: false}
	if err := store.DB.Create(&request).Error; err != nil {
		t.Fatal(err)
	}
	notifications, handled, err := game.Execute("group", receiver.AccountID, mustParse(t, "通知"))
	if err != nil || !handled || !strings.Contains(notifications.Content, "条未读") || !strings.Contains(notifications.Content, "【留言】") || !strings.Contains(notifications.Content, "【仙缘】") || hasGlobalPagination(notifications) {
		t.Fatalf("notification inbox: handled=%v err=%v result=%+v", handled, err, notifications)
	}
	var persistedRequest model.SocialMessage
	if err := store.DB.First(&persistedRequest, request.ID).Error; err != nil || persistedRequest.Read {
		t.Fatalf("viewing notifications consumed actionable request: row=%+v err=%v", persistedRequest, err)
	}
	cleared, handled, err := game.Execute("group", receiver.AccountID, mustParse(t, "清理已读通知"))
	if err != nil || !handled || !strings.Contains(cleared.Content, "不会被误删") {
		t.Fatalf("clear notifications: handled=%v err=%v result=%+v", handled, err, cleared)
	}
	if err := store.DB.First(&persistedRequest, request.ID).Error; err != nil {
		t.Fatalf("actionable request was deleted: %v", err)
	}
}
