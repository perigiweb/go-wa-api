package service

import (
	"log"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/perigiweb/go-wa-api/internal/store"
	"github.com/perigiweb/go-wa-api/internal/store/entity"
)

type WAEventHandler struct {
	client    *whatsmeow.Client
	handlerId uint32
	repo      *store.Repo
	uDevice   *entity.Device
}

func registerWAEventHandler(client *whatsmeow.Client, repo *store.Repo, uDevice *entity.Device) {
	var e = WAEventHandler{
		client:  client,
		repo:    repo,
		uDevice: uDevice,
	}

	e.register()
}

func (e *WAEventHandler) register() {
	e.handlerId = e.client.AddEventHandler(e.handler)
}

func (e *WAEventHandler) handler(evt interface{}) {
	log.Printf("WA Event Handler: %T\n\n", evt)

	switch v := evt.(type) {
	case *events.Connected, *events.PushNameSetting:
		if len(e.client.Store.PushName) == 0 {
			return
		}
		// Send presence available when connecting and when the pushname is changed.
		// This makes sure that outgoing messages always have the right pushname.
		err := e.client.SendPresence(types.PresenceAvailable)
		if err != nil {
			log.Println(err.Error())
		} else {
			log.Println("Marked self as available")
		}

		err = e.repo.UpdateConnected(true, e.uDevice.Id)
		if err != nil {
			log.Println(err.Error())
			return
		}

	case *events.PairSuccess:
		log.Println("Pair Success")
		err := e.repo.UpdateJID(v.ID, e.uDevice.Id)
		if err != nil {
			log.Println(err.Error())
			return
		}

	case *events.Message:
		log.Printf("Receive New Message: %+v\n", v)

	case *events.Receipt:
		log.Printf("Received a receipt: %+v\n", v)
		// May its a broadcast msg,
		if v.Type == types.ReceiptTypeRead {
			_ = e.repo.UpdateBroadcastMessageReceipt(v.MessageIDs, "read")
		} else if v.Type == types.ReceiptTypeDelivered {
			_ = e.repo.UpdateBroadcastMessageReceipt(v.MessageIDs, "delivered")
		}

	case *events.OfflineSyncCompleted:
		log.Printf("OfflineSyncCompleted!: %+v\n", v)

	case *events.HistorySync:
		log.Printf("HistorySync!: %+v\n", v)

	}
}
