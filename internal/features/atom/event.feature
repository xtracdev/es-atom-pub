Feature: Feed item retrieval
  Scenario:
    Given an event id exposed via a feed
    When I retrieve the event by its id
    Then the event detail is returned
    And cache headers indicate the resource is cacheable

