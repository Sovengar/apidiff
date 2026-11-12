# language: en
Feature: Detect OpenAPI breaking changes between two API contracts

  As a developer maintaining a Spring Boot API that publishes its contract at
  /v3/api-docs
  I want to compare a baseline contract with the current one
  So that a change which breaks consumers is visible before it reaches CI

  The tool is `apidiff`: one static binary, two commands. It never gates unless
  asked: the default is report-only.

  Background:
    Given two OpenAPI documents: a baseline and a revision
    And the baseline declares paths /widgets (GET, POST) and /widgets/{id} (GET)
    And the revision removes GET /widgets/{id}, makes POST /widgets require a
      new property "color", adds GET /widgets/search, and adds an optional
      request property "notes"

  Scenario: A removed path and a newly required request property are breaking
    When I run "apidiff check" with those two documents
    Then the exit code is 0
    And the report lists a breaking change with rule id starting
      "api-path-removed" for path "/widgets/{id}"
    And the report lists a breaking change with rule id
      "request-property-became-required" for path "/widgets"
    And each breaking change names a JSON pointer under "/paths/"

  Scenario: Additions are reported as non-breaking
    When I run "apidiff check" with those two documents
    Then the report lists a non-breaking change with rule id "endpoint-added"
      for path "/widgets/search"
    And the report lists a non-breaking change with rule id
      "new-optional-request-property"

  Scenario: A purely additive change has no breaking entries
    Given a baseline and a revision where the revision only adds
      GET /owners
    When I run "apidiff check"
    Then the exit code is 0
    And the report has zero breaking entries
    And the report has at least one non-breaking entry

  Scenario: Identical documents report no contract change
    Given the same document as baseline and revision
    When I run "apidiff check"
    Then the exit code is 0
    And the report says there are no contract changes

  Scenario: Fail-on turns the check into a gate
    When I run "apidiff check --fail-on ERR" with a breaking change
    Then the exit code is 1
    When I run "apidiff check --fail-on ERR" with only additive changes
    Then the exit code is 0

  Scenario: Rule ids can be ignored
    When I run "apidiff check --ignore endpoint-added" with an additive change
    Then the report does not list "endpoint-added"

  Scenario: A malformed revision is one actionable error, not a crash
    Given a revision document that is not valid YAML or JSON
    When I run "apidiff check"
    Then the exit code is 2
    And stderr names the offending document
    And no stack trace is printed

  Scenario: A missing file is an actionable error
    When I run "apidiff check" with a path that does not exist
    Then the exit code is 2
    And stderr names the path

  Scenario: JSON output is machine-readable and free of terminal colour
    When I run "apidiff check --format json"
    Then stdout is one JSON document with fields "base", "revision",
      "breaking", "nonBreaking"
    And no field contains an ANSI escape sequence

  Scenario: Markdown output is a report for a ticket
    When I run "apidiff check --format markdown"
    Then stdout contains a heading "API contract"
    And it lists the breaking entries with their rule id and pointer
    And it does not list non-breaking entries individually

  Scenario: Capture fetches a live contract
    Given a running server that serves /v3/api-docs
    When I run "apidiff capture --url URL --out baseline.json"
    Then the exit code is 0
    And the file contains pretty-printed JSON
    And a second capture refuses to overwrite without --force

  Scenario: Capture rejects a response that is not an OpenAPI document
    Given a URL that returns HTML
    When I run "apidiff capture --url URL --out baseline.json"
    Then the exit code is 2
    And no file is written
    And stderr says the response is not an OpenAPI document

  Scenario: A revision can be a live URL
    Given a baseline file and a running server
    When I run "apidiff check --base baseline.json --rev URL"
    Then the exit code is 0 and the report is produced

  Scenario: Usage errors never print a stack trace
    When I run "apidiff check" without --base
    Then the exit code is 2
    And stderr shows the usage for the check command
