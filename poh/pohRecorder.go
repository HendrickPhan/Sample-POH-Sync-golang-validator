package poh

import (
	"fmt"

	"example_poh.com/config"
)

func (recorder *POHRecorder) AddBlock(block POHBlock) {
	if block.Count < recorder.StartBlockCount {
		// skip block not in count range. May skip to far in future block?
		return
	}

	branchIdx := recorder.findBranchIdxForNewBlock(block)
	if branchIdx >= 0 {
		if int64(len(recorder.Branches[branchIdx].Blocks)) > block.Count-recorder.StartBlockCount {
			fmt.Printf("Here 1")
			// mean already have block with same count in this branch
			// so we have to fork (create new branch with same previous data for this block)
			// get previos blocks
			blocks := recorder.Branches[branchIdx].Blocks[:block.Count-recorder.StartBlockCount]
			// add new block
			blocks = append(blocks, block)
			// create new branch
			recorder.Branches = append(recorder.Branches, recorder.createBranch(blocks))
		} else {
			// just append new block to branch
			recorder.Branches[branchIdx].Blocks = append(recorder.Branches[branchIdx].Blocks, block)
		}

		// some update needed after insert a block
		recorder.updateMainBranch()
		recorder.updateStartCount()
		recorder.removeOldBlockFromBranches()
	} else {
		if block.Count == recorder.StartBlockCount {
			recorder.Branches = append(recorder.Branches, recorder.createBranch([]POHBlock{block}))
			recorder.updateMainBranch()

		}
	}
}

func (recorder *POHRecorder) GetMainBranchLastBlock() POHBlock {
	mainBranch := recorder.Branches[recorder.MainBranchIdx]
	return mainBranch.Blocks[len(mainBranch.Blocks)-1]
}

func (recorder *POHRecorder) findBranchIdxForNewBlock(block POHBlock) int {
	blockParentHash := block.Ticks[0].Hashes[0].LastHash
	idxOfParentBlock := block.Count - recorder.StartBlockCount - 1
	for i, branch := range recorder.Branches {
		if branch.Blocks[idxOfParentBlock].Hash.Hash == blockParentHash {
			return i
		}
	}
	return -1
}

func (recorder *POHRecorder) createBranch(blocks []POHBlock) POHBranch {
	totalTransaction := 0
	totalLeaderBlock := 0
	for _, block := range blocks {
		totalTransaction += block.GetTotalTransaction()
		if block.Type == "leader" {
			totalLeaderBlock++
		}
	}
	branch := POHBranch{
		Blocks:           blocks,
		TotalTransaction: totalTransaction,
		TotalLeaderBlock: totalLeaderBlock,
	}
	return branch
}

func (recorder *POHRecorder) updateMainBranch() {
	for v, branch := range recorder.Branches {
		if branch.TotalTransaction > recorder.Branches[recorder.MainBranchIdx].TotalTransaction {
			recorder.MainBranchIdx = v
		}
		if branch.TotalTransaction == recorder.Branches[recorder.MainBranchIdx].TotalTransaction {
			if branch.TotalLeaderBlock > recorder.Branches[recorder.MainBranchIdx].TotalLeaderBlock {
				recorder.MainBranchIdx = v
			}
		}
	}
}

func (recorder *POHRecorder) updateStartCount() {
	mainBranch := recorder.Branches[recorder.MainBranchIdx]
	lastBlock := mainBranch.Blocks[len(mainBranch.Blocks)-1]
	if lastBlock.Count > int64(config.AppConfig.BlockStackSize) {
		recorder.StartBlockCount = lastBlock.Count - int64(config.AppConfig.BlockStackSize)
	}
}

func (recorder *POHRecorder) removeOldBlockFromBranches() {
	for i := range recorder.Branches {
		totalRemovableBlock := recorder.StartBlockCount - recorder.Branches[i].Blocks[0].Count
		if totalRemovableBlock > 0 {
			recorder.Branches[i].Blocks = recorder.Branches[i].Blocks[totalRemovableBlock:]
		}
	}
}
