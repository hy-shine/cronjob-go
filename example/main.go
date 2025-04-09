package main

import (
	"fmt"
	"time"

	"github.com/hy-shine/cronjob-go"
)

func main() {
	cron, _ := cronjob.New(
		cronjob.WithEnableSeconds(),
	)
	cron.Add("job1", "*/2 * * * * *", func() error {
		fmt.Printf("running job1 at %s\n", time.Now().Local().Format("2006-01-02 15:04:05"))
		return nil
	})
	cron.Add("job2", "*/5 * * * * *", func() error {
		fmt.Printf("running job2 at %s\n", time.Now().Local().Format("2006-01-02 15:04:05"))
		return nil
	})

	cron.Start()
	defer cron.Stop()

	ticker := time.NewTimer(10 * time.Second)
	go func() {
		<-ticker.C
		cron.Upsert("job2", "*/3 * * * * *", func() error {
			fmt.Printf("changed: running job2 at %s\n", time.Now().Local().Format("2006-01-02 15:04:05"))
			return nil
		})
		ticker.Stop()
	}()

	select {}
}
