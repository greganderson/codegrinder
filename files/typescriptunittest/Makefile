.PHONY: test grade setup clean

setup:
	npm install

test: setup
	npx jest --config=tests/jest.config.ts --no-cache

grade: setup
	rm -f test_detail.xml
	JEST_JUNIT_OUTPUT_DIR="./" JEST_JUNIT_OUTPUT_NAME="test_detail.xml" npx jest --config=tests/jest.config.ts --no-cache --reporters=default --reporters=jest-junit

clean:
	rm -rf node_modules dist coverage test_detail.xml
