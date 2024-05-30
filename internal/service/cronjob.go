package service

import (
	"log"
	"time"

	"github.com/go-co-op/gocron/v2"
)

func (s *Service) CronJobs(c gocron.Scheduler) {
	log.Print("Cron Scheduler...")

	/*
	 * Cron jobs to check contact phone is in WhatsApp or not
	 */
	c.NewJob(
		gocron.DurationRandomJob(3*time.Minute, 9*time.Minute),
		gocron.NewTask(func() {
			log.Println("Cron run at: ", time.Now())
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
								log.Printf("Error: %s", err.Error())
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

	/*
	 * Cronjob for sending broadcast message
	 */
	c.NewJob(
		gocron.DurationRandomJob(45*time.Minute, 55*time.Second),
		gocron.NewTask(func() {
			broadcastToSend, err := s.Repo.GetBroadcastToSend()
			if err == nil {
				log.Printf("Broadcast: %v", broadcastToSend)
				if broadcastToSend.Recipient != nil {
					/*
					 * Insert recipient to db
					 */
					err = s.Repo.InsertBroadcastRecipient(broadcastToSend.Recipient)
					if err == nil {
						/*
						 * Send Message
						 */
					} else {
						log.Printf("Error: %s", err.Error())
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

	c.Start()
}
