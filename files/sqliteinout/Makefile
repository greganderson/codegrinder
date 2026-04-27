.SUFFIXES:
.SUFFIXES: .db .sql .actual .expected .xml

STEPPER_SUFFIX=sql
STEPPER_INDIR=.
STEPPER_OUTDIR=outputs
STEPPER_CMD=sqlite3 database.db
STEPPER_DIR=inputs
STEPPER_TIMEOUT=30.0
STEPPER_POSTERRORLINES=15

export STEPPER_SUFFIX STEPPER_INDIR STEPPER_OUTDIR STEPPER_CMD
export STEPPER_DIR STEPPER_POSTERRORLINES STEPPER_TIMEOUT

all:	step

grade:	$(HOME)/.sqliterc
	@rm -f database.db test_detail.xml outputs/*.actual
	@for x in $(shell ls .*.sql); do sqlite3 database.db < $$x > /dev/null; done
	@python3 lib/grader

step:	$(HOME)/.sqliterc transcripts
	@rm -f database.db
	@for x in $(shell ls .*.sql); do sqlite3 database.db < $$x > /dev/null; done
	@python3 lib/stepper
	@rm -rf $$STEPPER_DIR

shell:	$(HOME)/.sqliterc
	@rm -f database.db
	@for x in $(shell ls .*.sql); do sqlite3 database.db < $$x > /dev/null; done
	sqlite3 database.db

$(HOME)/.sqliterc:	lib/.sqliterc
	cp lib/.sqliterc $(HOME)/

setup:
	sudo apt install -y icdiff make python3 sqlite3

transcripts:
	@mkdir -p $$STEPPER_DIR
	@for f in outputs/*.expected; do \
		base=$$(basename $$f .expected); \
		{ printf '< .read %s.sql\n' $$base; sed 's/^/> /' $$f; } > inputs/$$base.transcript; \
	done

clean:
	rm -f test_detail.xml database.db outputs/*.actual
