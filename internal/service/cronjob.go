package service

import (
	"log"
	"math/rand"
	"time"

	"github.com/go-co-op/gocron/v2"
)

func (s *Service) CronJobs(c gocron.Scheduler) {
	log.Print("Cron Scheduler...")

	/*
	 * Cron jobs to check contact phone is in WhatsApp or not
	 */
	checkPhoneJob, _ := c.NewJob(
		gocron.DurationRandomJob(3*time.Minute, 5*time.Minute),
		gocron.NewTask(func() {
			contact, err := s.Repo.GetRandomContactNotInWA()

			if err == nil {
				log.Printf("Check phone: %s", contact.Phone)
				devices, _ := s.Repo.GetDevicesByUserId(contact.UserId)
				if len(devices) > 0 {
					device := devices[0]
					phones := []string{contact.Phone}

					results, err := s.CheckPhone(device.Id, phones)
					log.Printf("Result: %+v", results)
					if err == nil {
						result := results[0]
						if result.IsIn {
							contact.InWA = 1
							if result.VerifiedName != nil {
								contact.VerifiedName = result.VerifiedName.Details.GetVerifiedName()
							}

							err = s.Repo.SaveUserContact(&contact)
							if err != nil {
								log.Printf("Error Save Contact (%d): %v", contact.Id, err)
							}
						} else {
							_ = s.Repo.DeleteUserContactById(contact.Id, contact.UserId)
						}
					} else {
						log.Printf("Error: %s", err.Error())
					}
				}
			} else {
				log.Printf("Error: %s", err.Error())
			}
		}),
	)

	n, err := checkPhoneJob.NextRun()
	log.Printf("CheckPhone Next Run: %v; Error: %v", n, err)

	/*
	 * Cronjob for sending broadcast message
	 */
	smnj, _ := c.NewJob(
		gocron.DurationRandomJob(9*time.Minute, 11*time.Minute),
		gocron.NewTask(func() {
			log.Println("Send Message Job...")

			broadcastToSend, err := s.Repo.GetBroadcastToSend()
			if err == nil && broadcastToSend != nil {
				log.Printf("Broadcast: %s, Recipient: %s (%s); Total Recipient: %d", broadcastToSend.Broadcast.CampaignName, broadcastToSend.Recipient.Name, broadcastToSend.Recipient.Phone, broadcastToSend.TotalRecipient)
				if broadcastToSend.Recipient != nil {
					/*
					* Insert recipient to db
					 */
					err = s.Repo.InsertBroadcastRecipient(broadcastToSend.Recipient)
					if err == nil {
						/*
						* Send Typing Presence and then Send Message after delay
						 */
						err = s.SendChatPresence(broadcastToSend.Broadcast.Device.Id, broadcastToSend.Recipient.Phone, "composing", "")
						if err == nil {
							delay := rand.Intn(5) + 5
							timer := time.After(time.Duration(delay) * time.Second)
							select {
							case m := <-make(chan int):
								log.Println("case make chan int...", m)
							case <-timer:
								log.Printf("timer ==> after %d seconds, send message...", delay)
								sendResponse, err := s.SendBroadcastMessage(broadcastToSend)
								log.Printf("SendResponse: %+v", sendResponse)
								if err == nil {
									err = s.Repo.UpdateSentStatus(broadcastToSend.Recipient.Id, "sent", sendResponse.ID, sendResponse.Timestamp)
									log.Printf("Error UpdateSentStatus: %v", err)
								} else {
									log.Printf("Error SendBroadcastMessage: %+v", err.Error())
								}

								if broadcastToSend.TotalRecipient == 1 {
									err = s.Repo.UpdateCompletedBroadcast(broadcastToSend.Broadcast.Id, true, time.Now())
									if err != nil {
										log.Printf("UpdateCompletedBroadcast Error: %s", err.Error())
									}
								}
							}
						} else {
							log.Printf("SendChatPresence Error: %s", err.Error())
						}
					} else {
						log.Printf("Error InsertBroadcastRecipient: %s", err.Error())
					}
				} else {
					err = s.Repo.UpdateCompletedBroadcast(broadcastToSend.Broadcast.Id, true, time.Now())
					if err != nil {
						log.Printf("UpdateCompletedBroadcast Error: %s", err.Error())
					}
				}
			} else {
				log.Printf("Error: %T %s", err, err.Error())
			}
		}),
	)
	smnr, err := smnj.NextRun()
	log.Printf("Send Msg Next Run: %v, Error: %v", smnr, err)

	c.Start()
}
