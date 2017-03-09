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

	syncChan := make(chan struct{})

	defer func(lock db.Lock) {
		syncChan <- struct{}{} // this relays to the first one to ensure the lock is freed
		s.client.Free(lock, true)
	}(lock)

	go func(v *db.Volume) {
		for {
			select {
			case <-syncChan:
				return
			default:
				lock := db.NewCreateOwner("mon0", v)
				c.Assert(s.client.Acquire(lock), IsNil)
			}
		}
	}(v)

	go func(v *db.Volume) {
		for {
			select {
			case <-syncChan:
				syncChan <- struct{}{}
				return
			default:
				lock := db.NewCreateOwner("mon1", v)
				c.Assert(s.client.Acquire(lock), NotNil)
			}
		}
	}(v)

	logrus.Info("Creating contention in etcd")
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
	c.Assert(s.client.AcquireWithTTL(lock, time.Second), IsNil)
	c.Assert(s.client.AcquireWithTTL(lock2, time.Second), NotNil)

	time.Sleep(2 * time.Second) // wait for ttl to expire

	c.Assert(s.client.AcquireWithTTL(lock2, time.Second), IsNil)
	s.client.Free(lock2, true)
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

	// stopChan is sent a signal on finish of the goroutine below, this defeats a
	// false positive in our locking system (if both channels are sent in
	// lockstep, one will win about 50% of the time)
	stopChan, err := s.client.AcquireAndRefresh(lock, 5*time.Second)
	c.Assert(err, IsNil)

	syncChan := make(chan struct{})

	defer func() { syncChan <- struct{}{} }()

	go func(v *db.Volume) {
		for {
			select {
			case <-syncChan:
				stopChan <- struct{}{}
				return
			default:
				lock := db.NewCreateOwner("mon1", v)
				c.Assert(s.client.Acquire(lock), NotNil)
			}
		}
	}(v)

	logrus.Info("Creating contention in etcd")
	time.Sleep(time.Minute)
}
