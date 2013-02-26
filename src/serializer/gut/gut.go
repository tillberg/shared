
package gut

import (

)

type Serializer struct {}

func (s *Serializer) UnmarshalBranch(bytes []byte) (*types.Branch, error) {
  return nil, nil
}

func (s *Serializer) UnmarshalFile(bytes []byte)   (*types.File,   error) {
  return nil, nil
}

func (s *Serializer) UnmarshalCommit(bytes []byte) (*types.Commit, error) {
  return nil, nil
}

func (s *Serializer) UnmarshalTree(bytes []byte)   (*types.Tree,   error) {
  return nil, nil
}

func (s *Serializer) MarshalBranch(branch *types.Branch) []byte, error {
  return nil, nil
}

func (s *Serializer) MarshalFile(file   *types.File)     []byte, error {
  return nil, nil
}

func (s *Serializer) MarshalCommit(commit *types.Commit) []byte, error {
  return nil, nil
}

func (s *Serializer) MarshalTree(tree   *types.Tree)     []byte, error {
  return nil, nil
}
