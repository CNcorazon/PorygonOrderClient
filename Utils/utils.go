package Utils

import "horizon/structure"

func FindLeader(proBlocks []structure.ProposalBlock) structure.ProposalBlock {
	if len(proBlocks) == 0 {
		return structure.ProposalBlock{} // Return an empty ProposalBlock if the slice is empty
	}

	minVrf := proBlocks[0].Vrf
	minIdx := 0

	for i, proBlock := range proBlocks {
		if proBlock.Vrf < minVrf {
			minVrf = proBlock.Vrf
			minIdx = i
		}
	}

	return proBlocks[minIdx]
}
