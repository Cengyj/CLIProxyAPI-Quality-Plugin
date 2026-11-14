# Source acknowledgements

The native C ABI scaffolding is adapted from the CLIProxyAPI host-model-callback example in the supplied 8.0.67 source archive. CLIProxyAPI is MIT licensed; its notice is retained in CPA_LICENSE. The public /v8 SDK is a separate dependency.

sub2api (https://github.com/ranxi2001/sub2api) commit ccf997c73868083d96c0bec56e71c1a34d26c98a was studied as an architectural reference. Its root LICENSE is LGPL-3.0. The candy question text in `quality/questiondata/candy-sub2api.txt` is copied verbatim from `CANDY_PROMPT` in `frontend/src/utils/intelligenceTest.ts` at that commit and remains under LGPL-3.0; it is not relicensed under this plugin's MIT license. No original copyright notice was present in that source file. The upstream license and the incorporated GPL-3.0 terms are retained in `licenses/sub2api-LGPL-3.0.txt` and `licenses/GPL-3.0.txt`. The copied text is supplied with the plugin source and embedded into the dynamic library; rebuilding the supplied module regenerates the embedded question.

The candy question ID is `logic-candy`, version `sub2api-ccf997c7-v1`. The frozen prompt adds the upstream integer-only delivery contract, and the reference answer 21 applies only to this exact version. Editing the question requires a custom question with its own review standard. The local solver test independently checks the minimum by enumerating the shape and flavor counts. Manual question tests do not grade automatically by default.

No sub2api Vue components, Go implementation, artwork, or UI assets were copied. The bounded constraint questions, their implementations, and both pelican-on-bicycle prompts are independently written. Legacy question IDs retain their existing prompts and versions for saved rules and history.

This plugin has no published GitHub repository. Its required metadata source field is explicitly local-workspace:CLIProxyAPI-Quality-Plugin; the delivered source and checksummed host baseline are authoritative for this build.


## CPA management UI color reference

The native browser UI adapts the semantic color roles from
https://github.com/router-for-me/Cli-Proxy-API-Management-Center at commit
`d0126752201a35ff999bd2a1d4741215542258a8`, specifically
`src/styles/themes.scss`. The warm-gray light, white, and warm-black dark
palettes inform the plugin's own page, surface, text, border, action, focus,
and status tokens. Selected shades are adjusted within those color families
for text and control contrast.

The plugin's information architecture, navigation, page composition, and
components are independently implemented in plain HTML, CSS, and JavaScript
ES modules. It does not copy the host's business menus or use React.

A read-only unauthenticated static fetch of the local deployed management
page on 2026-11-14 confirmed its title and seven palette values against the
fixed reference. The HTML SHA-256 was
`650ca63b43487bcf2ebda6e6d66b8ac8429e5c1715c5f00f50405e9addf0bf70`.
This comparison does not establish the deployed frontend's exact Git commit.
The original CPA 8.0.67 host source reference remains unchanged.

MIT License

Copyright (c) 2026 Router-For.ME

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
