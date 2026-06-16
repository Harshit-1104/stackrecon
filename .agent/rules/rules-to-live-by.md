---
trigger: always_on
---

---
trigger: always_on
---

- Don't create .md files for the work you do like walkthroughs, readme's, implementation guide, etc unless specifically asked to do so.
- While writing code, always analyse how code is already being written in this repo and follow it's coding style and patterns instead of doing something alien.
- Don't add useless or very generic comments everywhere. Add comments only when asked to do so.
- Don't try to check for errors by building files, instead use go vet.
- When writing new tests, always verify the existing tests in this repo to understand how to write tests here.
- Always write extensive asserts for tests
- DO NOT run the scripts and code you write yourself unless specifically asked to do so. Instruct the human to run it always even when it's not clear whether you should run it or not.
- Whenever creating pipelines, if possible, always try to add checkpointing so rerunning the pipeline is not redundant.
- If you want to debug or run some code or a section of code, create a test file, run that, and then delete the test file after you've done your work.