package volplugin

func (dc *DaemonConfig) addStopChan(name string, stopChan chan struct{}) {
	dc.lockStopChanMutex.Lock()
	dc.lockStopChans[name] = stopChan
	dc.lockStopChanMutex.Unlock()
}

func (dc *DaemonConfig) removeStopChan(name string) {
	dc.lockStopChanMutex.Lock()
	if test, ok := dc.lockStopChans[name]; ok && test != nil {
		dc.lockStopChans[name] <- struct{}{}
	}
	delete(dc.lockStopChans, name)
	dc.lockStopChanMutex.Unlock()
}
