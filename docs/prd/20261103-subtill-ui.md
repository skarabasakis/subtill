# Subtill UI

This document describes the UI for the Subtill desktop application.

Convention:
- an html tag is represented as a `<tag/>` with the tag name in lowercase.
- a component that is available in shadcn registries is represented as `@shadcn/{component name}` or more generally as `{registry name}/{component name}`.

## Launcher

The Launcher is the screen that appears when the application is launched without a current database. The launcher is composed of the following elements:

Top section: A title "Subtill". Below it, a subtitle "Track your online subscriptions with ease". Below the subtitle, two side by side `@shadcn/button` items:
- "New Database" (primary): Triggers the "New Database" modal.
- "Open Database" (secondary): Opens the file dialog to select a database file.

Bottom section: A vertically stacked list of all databases created through the desktop application. Each database is represented as an `@shadcn/item` bearing the following elements:
- Content:
  - Title: The name of the database, followed by a primary `@shadcn/badge` item showing the number of subscriptions in the database.
  - Description: The filesystem path to the database.
- Actions: A three-dot menu with the following options that reveals the following `@shadcn/dropdownMenu` items when clicked:
  - "Hide Database": Removes the database from the launcher list.
  - "Delete Database": Deletes the database from the filesystem and removes it from the launcher list.

### New Database Modal

A `@shadcn/dialog` containing a `<form/>` with the following `@shadcn/field` items:
- "Display Name": An `@shadcn/input` field for the display name of the database.
- "Currency": A `@shadcn/comboBox` menu for the currency of the database, populated from a list of supported currencies embedded in the application.

When submitted, the form triggers the save file dialog with a default name that matches the database name. Once the file is created, an entry is added to the launcher list with the chosen filesystem path.

## SubscriptionManager

The SubscriptionManager appears when a database is opened and provides all the functionality to manage the subscriptions in the database. It is split into four screens, navigable from a sidebar on the left:
- "Subscriptions": A searchable, sortable, filterable list of all subscriptions in the database, with CRUD operations for each subscription and barebones aggregate reporting.
- "Payments": A filterable list of payments associated with all database subscriptions.
- "Reports": A list of all reports in the database.
- "Reminders": A list of all reminders in the database.
- "Settings": A settings screen to configure the opened database.

The sidebar also incorporates a "Switch Database" button that closes the current database and returns to the Launcher.

### Subscriptions Screen

The subscription screen provides the following views:
- Subscriptions List: A searchable, sortable, filterable list of all subscriptions in the database, with CRUD operations for each subscription and barebones aggregate reporting.
- Subscription Details: A view of a single subscription with its details, including billing cycles, payments, reminders and barebones aggregate reporting for the subscription.

#### Subscriptions List View

Four sections, laid out vertically:
- Toolbar (top)
- Table controls (middle, above the data table)
- Data table (middle)
- Mini-dash (bottom)

##### Toolbar
- On the left: An `<h1/>` with the display name of the opened database, for example "Personal Subscriptions".
- On the right: A `@shadcn/button` item with the label "New Subscription" that triggers the "Add Subscription" drawer.

###### Add Subscription Drawer

Contains a `<form/>` with the following `@shadcn/field` items:
- "Service": A group of two side by side controls:
  - An icon button, where the icon is automatically set from the domain extracted from the subscription name, using the part of the name before the first slash as the domain.
  - An `@shadcn/input` field for the name of the subscription. If the name contains a slash character, that automatically sets the group to the part of the name before the first slash.
  The field performs a live search of the database for subscriptions with names matching the input. If the user picks an existing subscription from the autocomplete list, the subscription icon and details are pre-filled in the form.
- "Group": A `@shadcn/comboBox` for the group of the subscription, populated from a list of current unique groups in the database.
- "Price": A group of the following side by side controls:
  - Currency: A `@shadcn/dropdownMenu` menu for the currency of the billing cost, populated from a list of supported currencies embedded in the application. Default is the database currency.
  - Cost: An `@shadcn/input` field for the billing cost of the subscription. inputmode is "decimal" with a placeholder of "0.00".
  - Frequency: Prefixed with the word "per" and a space, then a `@shadcn/dropdownMenu` menu with the following options: "month", "quarter (3 months)", "trimester (4 months)", "semester (6 months)", "year", "2 years", "3 years", "5 years", "10 years". Default is "month".
  - Tags: A `@formcn/tag-input` field for the tags of the subscription.
- "Frequency": A `@shadcn/comboBox` menu for the frequency of the subscription, populated from a list of supported frequencies embedded in the application.
- "Start On": A `@shadcn/datePicker` field for the start date of the subscription.
- "Duration": A `@shadcn/radioGroup` menu for the duration of the subscription, with options:
  - "Open-ended"
  - "Fixed": "N months/years" where "N months/years" is a custom numeric `@shadcn/input` field that renders as `(-) N months (+)` or `(-) N years (+)`, depending on the current value of the "Frequency" field. N is the duration of the subscription, a multiple of the current value of the "Frequency" field, with min value equal to the current value of the "Frequency" field, no max value and step equal to the current value of the "Frequency" field. When the Frequency field is changed, the field resets to the default, which is equal to 1 times the current value of the "Frequency" field.

##### Table controls

- Search box (left aligned): A `@shadcn/input` field for live search, magnifying glass icon on the left, rounded corners.
- Table controls (right aligned): A row of the following side-by-side controls:
  - Group Filter: A `@shadcn/dropdownMenu` labelled "Group: X" with options labelled "Any", plus one option corresponding to each unique group in the database. Hidden unless there is at least one group in the database.
  - Tags Filter: A `@shadcn/dropdownMenu` labelled "Tag: X, Y" with options labelled "Any", plus one option corresponding to each unique group in the database. Hidden unless there is at least one group in the database.
  - Status Filters: A `@shadcn/button` item with the label "Show: X, Y" that opens a `@shadcn/dropdownMenu` menu with three groups of options:
    - Status: An inline `@shadcn/toggleGroup` menu with mutually exclusive options: "Any", "Ongoing", "Ended" and "Scheduled". Default is "Any", which means no filter is applied.
    - Duration: An inline `@shadcn/toggleGroup` with mutually exclusive options: "Any", "Open-ended", "Fixed". Default is "Any", which means no filter is applied.
  - Sort `@shadcn/buttonGroup`:
    - Sort direction: A `@shadcn/button` item labeled with an up/down arrow icon that toggles the sort direction between ascending and descending.
    - Sort field: A `@shadcn/button` item with the label "Sort: X" that triggers the sorting of the table by the column X. Opens a `@shadcn/dropdownMenu` menu with the following options, where the active option has a checkmark icon on the left:
      - Date added (default)
      - Name
      - Cost
      - Next payment date
      - End date
  - Columns: A gear icon that opens a `@shadcn/drawer` with a list of checkbox options that correspond to the columns in the table. Each subscription field is represented as a column. The columns can be dragged and dropped to reorder them.

Note: The current state of the table controls is persisted by the backend in the application's configuration directory.

##### Data table

A `@shadcn/table` item with the following columns toggled on by default, in the following order:
- Status: Renders as a colored utf8 character icon corresponding to the status of the subscription:
  - grey color: ○ (ended),
  - green color: ● (active, open-ended), ◑ (active, fixed-term)
  - purple color: ◆ (scheduled, open-ended), ⬗ (scheduled, fixed-term)
- Name: the icon and name of the subscription
- Next payment on: Renders as e.g. "Nov 15, 2027", year displayed only if the date is not in the current year. Blank if null.
- Cost: Renders in the database currency, with the billing cost rendered in parentheses in the currency of the subscription, if it differs.
- Frequency: Renders as e.g. "m", "3m", "y", "2y" for 1, 3, 12 and 24 months respectively.
- Start On: Renders as e.g. "Nov 15, 2027", year displayed only if the date is not in the current year. Blank if null.
- End On: Renders as e.g. "Nov 15, 2027", year displayed only if the date is not in the current year. Blank if null.
- Tags: Renders as a comma-separated list of the tags of the subscription.

Clicking on a row opens the subscription details screen.

The subscription status (ongoing, ended, scheduled) is also represented through the background color of the row:
- ended: light grey background
- ongoing: light green background
- scheduled: light purple background

The table body is vertically scrollable.

##### Mini-dash
A mini-dash with side-by-side counters for the following: Number of subscriptions, Cost this month, Cost this year. Side by side and aligned to the left. Each counter is a `@shadcn/card` item with the counter value in large type as the title and the counter label as the description. The counters refresh automatically to reflect the currently applied filter.


#### Subscription Details View

Four sections, laid out top to bottom:
- Toolbar
- Mini-dash
- Tabs: User can switch between the following tabs:
  - Billing Periods
  - Payments
  - Reminders

##### Toolbar
- On the left: A back button (arrow left icon) that closes the subscription details screen and returns to the subscriptions list screen, followed by the subscription icon and name.
- On the right: A three dots icon that opens a `@shadcn/dropdownMenu` menu with the following options:
  - Buttons for the primary operations of the subscription, which depend on the status of the subscription:
    - For ongoing subscriptions:
      - When open-ended:
        - "Change price": Applies to non-ended subscriptions only. Triggers the `subscription.start` operation and asks the user for the new price and frequency.
        - "Cancel": Triggers the `subscription.cancel` operation and asks the user for the duration until the end date: "at the end of the current month, quarter, trimester, semester, year, N-month period,N-year period" or "after X months/years", where X is multiple of the current frequency. A preview of the end date is displayed in the button.
      - When fixed-term:
        - "Uncancel": Triggers the `subscription.resume` operation.
    - For ended subscriptions:
      - "Renew": Triggers the `subscription.start` operation, asking the user for the start date and subscription cost.
  - A three dots icon that opens a `@shadcn/dropdownMenu` menu with the rest of the operations that can be performed on the subscription, including:
    - "Add Billing Period": Opens the "Add Billing Period" drawer.
    - "Edit": Opens the "Edit Subscription" drawer.
    - "Delete": Triggers the `subscription.delete` operation.

##### Mini-dash

Counters for the following:
- left: cost, next payment date, due this year
- right: lifetime duration, lifetime cost

##### Billing Periods Tab

A horizontally scrollable list of `@shadcn/card` items with the following fields.
- Start On: Renders as e.g. "Nov 15, 2027", year displayed only if the date is not in the current year. Blank if null.
- End On: Renders as e.g. "Nov 15, 2027", year displayed only if the date is not in the current year. Blank if null.
- Cost: Renders in the database currency, with the billing cost rendered in parentheses in the currency of the subscription, if it differs.
- Frequency: Renders as e.g. "m", "3m", "y", "2y" for 1, 3, 12 and 24 months respectively.

The cards are ordered by start date in ascending order, and the list opens scrolled all the way to the right end. Each card is about 300px wide, so a standard window fits 3 cards. Each card has a color gradient background that goes from white to light grey/green/purple, depending on the status of the billing period. There is an 8px gap between cards, unless they represent consecutive billing periods, in which case the gap is 0px.

At the top left of each card, a three dots icon that opens a `@shadcn/dropdownMenu` menu with the following options:
- "Edit": Opens the "Edit Billing Period" drawer.
- "Delete": Triggers the `billing_period.delete` operation.

##### Payments Tab

Above the table a year selector, rendered as "(<) 2026 (>)", where the year is the current year. When the user selects a different year, the table is filtered to only show payments for that year.

A `@shadcn/table` item with the following columns, ordered by date in ascending order:
- Date: Renders as e.g. "Nov 15, 2027", year displayed only if the date is not in the current year. Blank if null.
- Cost: Renders in the database currency, with the billing cost rendered in parentheses in the currency of the subscription, if it differs.

The next payment row is highlighted.
The last row is the total cost for the year, rendered as "Total: X", where X is the total cost for the year.
The last row is bold.

##### Reminders Tab

A list of cards, each representing a reminder, with the following elements:
- Date: Renders as e.g. "Nov 15, 2027", year displayed only if the date is not in the current year. Blank if null.
- Description: Renders as e.g. "Payment due", "Billing period due", "Subscription due".
- Status: Renders as e.g. "Pending", "Completed", "Cancelled".
with a three dots icon that opens to show a delete command.

Blank state: A card with the text "No reminders set for ${subscription name}" and a button to add a new reminder.
