Feature: Recommended ruleset
  Linter applies every approved rule end-to-end. Each rule catches its
  violation and stays quiet on a clean document.

  Background:
    Given the bundled recommended ruleset is loaded

  Scenario Outline: Missing field fails the matching error rule
    Given a fully populated OpenRPC document covering every approved rule
    And the document is missing "<path>"
    When I run the linter
    Then the linter exits non-zero
    And the lint output should mention rule "<rule>"

    Examples:
      | rule               | path                   |
      | info-description   | info.description       |
      | method-description | methods[0].description |
      | method-errors      | methods[0].errors      |
      | method-examples    | methods[0].examples    |

  Scenario Outline: Missing field warns for the matching rule
    Given a fully populated OpenRPC document covering every approved rule
    And the document is missing "<path>"
    When I run the linter
    Then the linter exits zero
    And the lint output should contain a warning for rule "<rule>"

    Examples:
      | rule                | path                                    |
      | info-license        | info.license                            |
      | method-summary      | methods[0].summary                      |
      | result-description  | methods[0].result.description           |
      | tag-description     | methods[0].tags[0].description          |
      | error-description   | methods[0].errors[0].description        |
      | example-description | methods[0].examples[0].description      |
      | schema-title        | methods[0].params[0].schema.title       |
      | schema-description  | methods[0].params[0].schema.description |
      | schema-type         | methods[0].params[0].schema.type        |

  Scenario Outline: Out-of-bounds value warns for the matching rule
    Given a fully populated OpenRPC document covering every approved rule
    And the document is mutated so that "<path>" <mutation>
    When I run the linter
    Then the linter exits zero
    And the lint output should contain a warning for rule "<rule>"

    Examples:
      | rule                  | path                         | mutation                         |
      | methods-non-empty     | methods                      | is set to an empty array         |
      | method-examples-min   | methods[0].examples          | is set to an empty array         |
      | param-count-limit     | methods[0].params            | is set to an array of 5 params   |
      | method-summary-length | methods[0].summary           | is set to a 121 character string |
      | param-summary-length  | methods[0].params[0].summary | is set to a 121 character string |

  Scenario: Fully populated document passes every recommended rule
    Given a fully populated OpenRPC document covering every approved rule
    When I run the linter
    Then the linter exits zero

  Scenario Outline: Rule stays quiet when field present and within bounds
    Given a fully populated OpenRPC document covering every approved rule
    When I run the linter
    Then the lint output should not mention rule "<rule>"

    Examples:
      | rule                  |
      | info-description      |
      | info-license          |
      | method-summary        |
      | method-description    |
      | method-errors         |
      | method-examples       |
      | method-examples-min   |
      | method-summary-length |
      | param-summary-length  |
      | param-count-limit     |
      | result-description    |
      | tag-description       |
      | error-description     |
      | example-description   |
      | schema-title          |
      | schema-description    |
      | schema-type           |
      | methods-non-empty     |
