// Copyright (c) 2016 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tokenizer_test

import (
	"github.com/projectcalico/libcalico-go/lib/selector/tokenizer"

	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var tokenTests = []struct {
	input    string
	expected []tokenizer.Token
}{
	{`a=="b"`, []tokenizer.Token{
		{tokenizer.TokLabel, "a"},
		{tokenizer.TokEq, nil},
		{tokenizer.TokStringLiteral, "b"},
		{tokenizer.TokEof, nil},
	}},

	{`a=="b"`, []tokenizer.Token{
		{tokenizer.TokLabel, "a"},
		{tokenizer.TokEq, nil},
		{tokenizer.TokStringLiteral, "b"},
		{tokenizer.TokEof, nil},
	}},
	{`label == "value"`, []tokenizer.Token{
		{tokenizer.TokLabel, "label"},
		{tokenizer.TokEq, nil},
		{tokenizer.TokStringLiteral, "value"},
		{tokenizer.TokEof, nil},
	}},
	{`a not in "bar" && !has(foo) || b in c`, []tokenizer.Token{
		{tokenizer.TokLabel, "a"},
		{tokenizer.TokNotIn, nil},
		{tokenizer.TokStringLiteral, "bar"},
		{tokenizer.TokAnd, nil},
		{tokenizer.TokNot, nil},
		{tokenizer.TokHas, "foo"},
		{tokenizer.TokOr, nil},
		{tokenizer.TokLabel, "b"},
		{tokenizer.TokIn, nil},
		{tokenizer.TokLabel, "c"},
		{tokenizer.TokEof, nil},
	}},
	{`a  not  in  "bar"  &&  ! has( foo )  ||  b  in  c `, []tokenizer.Token{
		{tokenizer.TokLabel, "a"},
		{tokenizer.TokNotIn, nil},
		{tokenizer.TokStringLiteral, "bar"},
		{tokenizer.TokAnd, nil},
		{tokenizer.TokNot, nil},
		{tokenizer.TokHas, "foo"},
		{tokenizer.TokOr, nil},
		{tokenizer.TokLabel, "b"},
		{tokenizer.TokIn, nil},
		{tokenizer.TokLabel, "c"},
		{tokenizer.TokEof, nil},
	}},
	{`a notin"bar"&&!has(foo)||b in"c"`, []tokenizer.Token{
		{tokenizer.TokLabel, "a"},
		{tokenizer.TokNotIn, nil},
		{tokenizer.TokStringLiteral, "bar"},
		{tokenizer.TokAnd, nil},
		{tokenizer.TokNot, nil},
		{tokenizer.TokHas, "foo"},
		{tokenizer.TokOr, nil},
		{tokenizer.TokLabel, "b"},
		{tokenizer.TokIn, nil},
		{tokenizer.TokStringLiteral, "c"},
		{tokenizer.TokEof, nil},
	}},
	{`a not in {}`, []tokenizer.Token{
		{tokenizer.TokLabel, "a"},
		{tokenizer.TokNotIn, nil},
		{tokenizer.TokLBrace, nil},
		{tokenizer.TokRBrace, nil},
		{tokenizer.TokEof, nil},
	}},
	{`a not in {"a"}`, []tokenizer.Token{
		{tokenizer.TokLabel, "a"},
		{tokenizer.TokNotIn, nil},
		{tokenizer.TokLBrace, nil},
		{tokenizer.TokStringLiteral, "a"},
		{tokenizer.TokRBrace, nil},
		{tokenizer.TokEof, nil},
	}},
	{`a not in {"a","B"}`, []tokenizer.Token{
		{tokenizer.TokLabel, "a"},
		{tokenizer.TokNotIn, nil},
		{tokenizer.TokLBrace, nil},
		{tokenizer.TokStringLiteral, "a"},
		{tokenizer.TokComma, nil},
		{tokenizer.TokStringLiteral, "B"},
		{tokenizer.TokRBrace, nil},
		{tokenizer.TokEof, nil},
	}},
}

var _ = Describe("Token", func() {

	for _, test := range tokenTests {
		test := test // Take copy for closure
		It(fmt.Sprintf("should tokenize %#v as %v", test.input, test.expected), func() {
			Expect(tokenizer.Tokenize(test.input)).To(Equal(test.expected))
		})
	}
})
