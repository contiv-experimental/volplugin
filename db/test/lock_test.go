package test

import (
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/db"
	. "gopkg.in/check.v1"
)

func (s *testSuite) TestLockAcquire(c *C) {
	copy := testPolicies["basic"].Copy()
	copy.(*db.Policy).Name = "policy1"
	c.Assert(s.client.Set(copy), IsNil)

	v, err := db.CreateVolume(&db.VolumeRequest{Policy: copy.(*db.Policy), Name: "test"})
	c.Assert(err, IsNil, Commentf("%v", v))
	c.Assert(s.client.Set(v), IsNil)

	lock := db.NewCreateOwner("mon0", v)

	path, err := lock.Path()
	c.Assert(err, IsNil)
	c.Assert(path, Equals, strings.Join([]string{lock.Prefix(), v.String()}, "/"))
	c.Assert(lock.Prefix(), Equals, "users/volume")
	c.Assert(s.client.Acquire(lock), IsNil)

	testUse := db.NewUse(v)
	c.Assert(s.client.Get(testUse), IsNil)

	// test that we can acquire the fetched lock.
	path, err = testUse.Path()
	c.Assert(err, IsNil)
	c.Assert(path, Equals, strings.Join([]string{lock.Prefix(), v.String()}, "/"))
	c.Assert(lock.Prefix(), Equals, "users/volume")
	c.Assert(s.client.Acquire(lock), IsNil)

	lock2 := db.NewCreateOwner("mon1", v)
	c.Assert(s.client.Acquire(lock2), NotNil)
	c.Assert(s.client.Free(lock2, false), NotNil)

	c.Assert(s.client.Free(lock, false), IsNil)
	c.Assert(s.client.Acquire(lock2), IsNil)
	c.Assert(s.client.Free(lock2, false), IsNil)

	c.Assert(s.client.Acquire(lock), IsNil)
	c.Assert(s.client.Free(lock2, true), IsNil)
}

func (s *testSuite) TestLockBattery(c *C) {
	copy := testPolicies["basic"].Copy()
	copy.(*db.Policy).Name = "policy1"
	c.Assert(s.client.Set(copy), IsNil)

	v, err := db.CreateVolume(&db.VolumeRequest{Policy: copy.(*db.Policy), Name: "test"})
	c.Assert(err, IsNil, Commentf("%v", v))
	c.Assert(s.client.Set(v), IsNil)

	lock := db.NewCreateOwner("mon0", v)
	c.Assert(s.client.Acquire(lock), IsNil)

	// go routine A should never free the lock.
	// go routine B should never succeed at acquiring it.

	syncChan1 := make(chan struct{})
	syncChan2 := make(chan struct{}, 1)

	defer func() {
		syncChan2 <- struct{}{} // this relays to the first one to ensure the lock is freed
		syncChan1 <- struct{}{} // this relays to the second one to terminate it
	}()

	go func(v *db.Volume) {
		lock := db.NewCreateOwner("mon0", v)
		for {
			select {
			case <-syncChan1:
				s.client.Free(lock, true)
				return
			default:
				c.Assert(s.client.Acquire(lock), IsNil)
			}
		}
	}(v)

	go func(v *db.Volume) {
		lock := db.NewCreateOwner("mon1", v)
		for {
			select {
			case <-syncChan2:
				return
			default:
				logrus.Debug("Attempting to acquire lock (should fail)")
				c.Assert(s.client.Acquire(lock), NotNil)
			}
		}
	}(v)

	logrus.Infof("Creating contention in %s", Driver)
	time.Sleep(time.Minute)
}

func (s *testSuite) TestLockTTL(c *C) {
	copy := testPolicies["basic"].Copy()
	copy.(*db.Policy).Name = "policy1"
	c.Assert(s.client.Set(copy), IsNil)

	v, err := db.CreateVolume(&db.VolumeRequest{Policy: copy.(*db.Policy), Name: "test"})
	c.Assert(err, IsNil, Commentf("%v", v))
	c.Assert(s.client.Set(v), IsNil)

	lock := db.NewCreateOwner("mon0", v)
	lock2 := db.NewCreateOwner("mon1", v)

	s.client.Free(lock, true)
	c.Assert(s.client.AcquireWithTTL(lock, 15*time.Second), IsNil)
	c.Assert(s.client.AcquireWithTTL(lock2, 15*time.Second), NotNil)

	time.Sleep(5 * time.Second)                                   // wait for ttl to expire. minimum timeout is 10s so we're testing that.
	c.Assert(s.client.AcquireWithTTL(lock2, time.Second), NotNil) // test in the middle so we're sure the lock isn't freed yet.
	time.Sleep(11 * time.Second)

	c.Assert(s.client.AcquireWithTTL(lock2, time.Second), IsNil)
	c.Assert(s.client.Free(lock2, false), IsNil)
}

func (s *testSuite) TestLockTTLRefresh(c *C) {
	copy := testPolicies["basic"].Copy()
	copy.(*db.Policy).Name = "policy1"
	c.Assert(s.client.Set(copy), IsNil)

	v, err := db.CreateVolume(&db.VolumeRequest{Policy: copy.(*db.Policy), Name: "test"})
	c.Assert(err, IsNil, Commentf("%v", v))
	c.Assert(s.client.Set(v), IsNil)

	lock := db.NewCreateOwner("mon0", v)
	s.client.Free(lock, true)

	stopChan, err := s.client.AcquireAndRefresh(lock, 15*time.Second)
	c.Assert(err, IsNil)

	sync := make(chan struct{})
	sync2 := make(chan struct{})

	// gocheck doesn't run very well in goroutines, so these panics are a
	// makeshift version of what gocheck actually does.
	go func(v *db.Volume) {
		for {
			select {
			case <-sync:
				lock := db.NewCreateOwner("mon1", v)

				time.Sleep(15 * time.Second)

				if err := s.client.Acquire(lock); err != nil {
					panic(err)
				}

				if err := s.client.Free(lock, false); err != nil {
					panic(err)
				}
				close(sync2)
				return
			default:
				lock := db.NewCreateOwner("mon1", v)
				if err := s.client.Acquire(lock); err == nil {
					panic("Acquired lock for mon1")
				}
			}
		}
	}(v)

	logrus.Infof("Creating contention in %s", Driver)
	time.Sleep(time.Minute)
	close(stopChan)
	close(sync)
	<-sync2
	logrus.Info("Lock successfully traded in background goroutine!")
}
