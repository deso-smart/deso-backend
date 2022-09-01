package main

import (
	"context"
	"fmt"
	"github.com/deso-smart/deso-backend/v2/scripts/tools/toolslib"
	"github.com/deso-smart/deso-core/v2/lib"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
	"sort"
	"time"
)

func main() {
	dirSnap := "$HOME/data_dirs/hypersync/final_nodes/runner_node"
	time.Sleep(1 * time.Millisecond)
	dbSnap, err := toolslib.OpenDataDir(dirSnap)
	if err != nil {
		fmt.Printf("Error reading db1 err: %v", err)
		return
	}
	snap, err, _ := lib.NewSnapshot(dbSnap, dirSnap, lib.SnapshotBlockHeightPeriod, false, false, &lib.DeSoMainnetParams, false)
	if err != nil {
		fmt.Printf("Error reading snap err: %v", err)
		return
	}
	snap.CurrentEpochSnapshotMetadata.SnapshotBlockHeight = 114000
	snap.Checksum.ResetChecksum()

	maxBytes := uint32(8 << 20)
	var prefixes [][]byte
	for prefix, isState := range lib.StatePrefixes.StatePrefixesMap {
		if !isState {
			continue
		}

		prefixes = append(prefixes, []byte{prefix})
	}
	sort.Slice(prefixes, func(ii, jj int) bool {
		return prefixes[ii][0] < prefixes[jj][0]
	})
	fmt.Println(prefixes)
	fmt.Printf("Checking prefixes: ")
	numProcesses := int64(1)
	sem := semaphore.NewWeighted(numProcesses)
	ctx := context.Background()

	lib.Mode = lib.EnableTimer
	timer := lib.Timer{}
	timer.Initialize()

	timer.Start("Compute checksum")
	for _, prefix := range prefixes {
		fmt.Printf("%v \n", prefix)
		if err := sem.Acquire(ctx, 1); err != nil {
			panic(errors.Wrapf(err, "Problem acquiring semaphore in the routine"))
		}

		go func(prefix []byte) {
			defer sem.Release(1)

			lastPrefix := prefix
			removeFirst := false
			for {
				entries, fullDb, err := lib.DBIteratePrefixKeys(dbSnap, prefix, lastPrefix, maxBytes)
				if err != nil {
					panic(fmt.Errorf("Problem fetching snapshot chunk (%v)", err))
				}
				if removeFirst {
					entries = entries[1:]
				}
				for _, entry := range entries {
					snap.AddChecksumBytes(entry.Key, entry.Value)
				}

				if len(entries) != 0 {
					lastPrefix = entries[len(entries)-1].Key
					removeFirst = true
				} else if fullDb {
					panic("Number of ancestral records should not be zero")
				}

				if !fullDb {
					break
				}
			}
		}(prefix[:])

		//time.Sleep(1 * time.Second)
		//fmt.Println("current operations:", snap.OperationChannel.GetStatus())
		//snap.WaitForAllOperationsToFinish()
		//checksumBytes, _ := snap.Checksum.ToBytes()
		//fmt.Println("prefix", prefix, "checksum:", checksumBytes)
	}
	if err := sem.Acquire(ctx, numProcesses); err != nil {
		panic(errors.Wrapf(err, "Problem acquiring semaphore after routines"))
	}

	fmt.Println("Finished iterating all prefixes")
	snap.WaitForAllOperationsToFinish()
	checksumBytes, _ := snap.Checksum.ToBytes()
	fmt.Println("Final checksum:", checksumBytes)

	timer.End("Compute checksum")
	timer.Print("Compute checksum")

}
