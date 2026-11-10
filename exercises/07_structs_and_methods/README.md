# 07 · Structs and methods

## Scenario

You are building the domain core of a small commerce and HR back office. It has
five pieces: a money type for prices and balances, a shopping cart, bank accounts,
an employee record importer, and a version type for the release tooling. Each one
is a struct whose invariants are enforced in one place: the constructor. Each one
exposes behaviour through methods. You choose a value or pointer receiver
for each method, and decide what the zero value means.

Files in this directory:

| File | Part | Type |
|------|------|------|
| `money.go` | 1 | `Money` |
| `cart.go` | 2 | `Product`, `Cart` |
| `account.go` | 3 | `Account`, `CheckingAccount`, `SavingsAccount` |
| `employee.go` | 4 | `Employee` |
| `version.go` | 5 | `Version`, `Part` |

Implement the stubs so that `go test` passes. The tests and this README are the
spec. Where they disagree, the tests win.

## Part 1 · Money: a value type with integer minor units

`Money` stores an `int64` count of minor units (cents, yen, fils) and a currency
code. Floats cannot represent `0.10` exactly, so money is never a float.

```go
func NewMoney(minor int64, currency string) (Money, error)
func ParseMoney(amount, currency string) (Money, error)
func (m Money) Minor() int64
func (m Money) Currency() string
func (m Money) IsZero() bool
func (m Money) Add(o Money) (Money, error)
func (m Money) Sub(o Money) (Money, error)
func (m Money) Compare(o Money) (int, error)
func (m Money) Scale(num, den int64) (Money, error)
func (m Money) Allocate(n int) ([]Money, error)
func (m Money) String() string
func (m Money) Format(f fmt.State, verb rune)
func (m Money) GoString() string
func (m Money) MarshalText() ([]byte, error)
func (m *Money) UnmarshalText(text []byte) error
```

Behaviour and edge cases:

- A currency is three upper-case ASCII letters. Anything else gives `ErrInvalidCurrency`.
- The minor-unit exponent depends on the currency. JPY has 0 digits, KWD has 3, and
  most others have 2. `JPY` `100` is `100 JPY`. `KWD` `1005` is `1.005 KWD`.
- `ParseMoney` accepts an optional sign, at least one integer digit, and an optional
  fraction. `5.` and `.5` are invalid (`ErrInvalidAmount`). Extra fractional digits
  are allowed only when they are all zero, so `1.500 USD` is `150`. A non-zero extra
  digit gives `ErrPrecision`. A value outside int64 gives `ErrOverflow`.
- `Add`, `Sub` and `Scale` check for overflow and return `ErrOverflow`. Mixing
  currencies gives `ErrCurrencyMismatch`. This includes the zero value `Money{}`,
  which has no currency.
- `Scale(num, den)` computes `m * num / den` with banker's rounding (round half to
  even). `den == 0` gives `ErrInvalidArgument`.
- `Allocate(n)` splits the amount into `n` shares that sum exactly to the original.
  Each share differs by at most one unit, and the first remainder units go to the
  first shares. `n < 1` gives `ErrInvalidArgument`. Negative amounts split by magnitude.
- The zero value `Money{}` prints as `0.00`. Its `MarshalText` fails with
  `ErrInvalidCurrency`.
- `Format` handles `%v`, `%s` (width and the `-` flag), `%d` (raw minor units) and
  `%#v`. Other verbs print `%!x(structs.Money=10.50 USD)`. `Formatter` takes
  precedence over `Stringer` and `GoStringer`.
- `MarshalText` and `UnmarshalText` use the form `10.50 USD`. `UnmarshalText` leaves
  the receiver unchanged on error.

## Part 2 · Product and Cart: immutable values and a zero-value cart

`Product` is an immutable catalogue value. Its fields are unexported, so only
`NewProduct` and `Restock` build one. `Cart` is a mutable aggregate that takes its
currency from its first product, or from `NewCart`.

```go
func NewProduct(sku, name string, price Money, quantity int) (Product, error)
func (p Product) SKU() string
func (p Product) Name() string
func (p Product) Price() Money
func (p Product) Quantity() int
func (p Product) InStock() bool
func (p Product) Restock(count int) (Product, error)

func NewCart(currency string) (*Cart, error)
func (c *Cart) Currency() string
func (c *Cart) Add(p Product, qty int) error
func (c *Cart) Remove(sku string, qty int) error
func (c *Cart) Quantity(sku string) int
func (c *Cart) Has(sku string) bool
func (c *Cart) Len() int
func (c *Cart) All() iter.Seq2[string, int]
func (c *Cart) Total() (Money, error)
func (c *Cart) String() string
```

Behaviour and edge cases:

- A SKU has 4 to 16 characters from `A-Z`, `0-9` and `-`, and starts with a letter or
  digit. Otherwise `ErrInvalidSKU`.
- The name is trimmed. It must be 1 to 100 runes, or `ErrInvalidName`.
- The price needs a valid currency (`ErrInvalidCurrency`). A negative price gives
  `ErrInvalidArgument`. A negative stock quantity gives `ErrInvalidQuantity`.
- `Restock` returns a copy and leaves the receiver unchanged. `count < 1` and stock
  overflow give `ErrInvalidQuantity`.
- The zero-value `Cart` is empty and has no currency. It adopts the currency of the
  first product added. `NewCart("usd")` gives `ErrInvalidCurrency`.
- `Add` with a product in another currency gives `ErrCurrencyMismatch` and leaves
  the cart unchanged. A zero `Product{}` gives `ErrInvalidSKU`. `qty < 1` gives
  `ErrInvalidQuantity`.
- Re-adding a SKU increases its quantity and keeps the first `Product` snapshot.
- `Remove` of more than the cart holds gives `ErrInsufficientQuantity` and leaves the
  cart unchanged. A SKU not in the cart gives `ErrNotInCart`. A line that reaches
  zero is deleted.
- `All` iterates in insertion order. `Total` sums price times quantity with checked
  arithmetic. An empty cart totals zero in its currency.
- `String` uses a pointer receiver: `Cart(currency="USD", items=3)`.

## Part 3 · Accounts: embedding, interfaces and one mutex

Both account types embed an unexported `ledger` that holds the owner, the balance
and a `sync.Mutex`. The mutex is part of the account and must never be copied. Accounts are
used through pointers. Both types implement `Account`. A value type does not.

```go
type Account interface {
	Owner() string
	Balance() Money
	Deposit(Money) error
	Withdraw(Money) error
}

func NewCheckingAccount(owner string, opening, overdraft Money) (*CheckingAccount, error)
func (a *CheckingAccount) Overdraft() Money

func NewSavingsAccount(owner string, opening Money, maxWithdrawals int) (*SavingsAccount, error)
func (s *SavingsAccount) MaxWithdrawals() int
func (s *SavingsAccount) WithdrawalsThisPeriod() int
func (s *SavingsAccount) ResetPeriod()
```

`Owner`, `Balance`, `Deposit` and `Withdraw` are promoted from `ledger`. Each
account type overrides `Withdraw`.

Behaviour and edge cases:

- The owner is trimmed. A blank owner gives `ErrInvalidOwner`. The opening balance
  must have a valid currency (`ErrInvalidCurrency`) and must not be negative.
- `Deposit` checks the currency first, so `Money{}` gives `ErrCurrencyMismatch`. A
  non-positive amount gives `ErrNonPositiveAmount`.
- A checking account allows withdrawals down to `-overdraft`. A zero-value overdraft
  means none. A negative overdraft gives `ErrInvalidArgument`. An overdraft in another
  currency gives `ErrCurrencyMismatch`.
- `ErrInsufficientFunds` means the amount exceeds balance plus overdraft.
- A savings account checks the period limit before the amount. It never overdraws.
  Only successful withdrawals count. Once the count reaches the limit,
  `ErrWithdrawalLimit`. A negative limit gives `ErrInvalidArgument`.
- Concurrent deposits and withdrawals keep the balance and count exact. Every
  operation holds the lock for its whole check-and-update.

## Part 4 · Employee: constructors, loose input and a non-comparable struct

`Employee` has unexported fields and is built only by constructors. It holds a slice,
so `==` does not compile. Use `Equal`.

```go
func NewEmployee(id int, name, email, department string, salary Money, skills []string) (Employee, error)
func NewEmployeeFromMap(data map[string]any) (Employee, error)
func NewEmployeeFromCSV(row []string) (Employee, error)
func NormalizeEmail(raw string) (string, error)
func (e Employee) ID() int
func (e Employee) Name() string
func (e Employee) Email() string
func (e Employee) Department() string
func (e Employee) Salary() Money
func (e Employee) Skills() []string
func (e Employee) Equal(o Employee) bool
```

Error contract:

- Field validation failures wrap `ErrInvalidEmployee`. This covers a non-positive ID,
  a blank name or department, an invalid salary, and unparseable values in the map or CSV.
- Email failures wrap `ErrInvalidEmail`. The email check comes from `NormalizeEmail`.
- A required key missing from a map record gives `ErrMissingField`.
- A CSV row without exactly seven columns gives `ErrCSVColumns`.

Behaviour and edge cases:

- `NewEmployee` trims the name and department, and both must be non-empty. The email
  is trimmed and lower-cased, then checked. The salary needs a valid currency and must
  not be negative.
- `NewEmployeeFromMap` requires `employee_id`, `name`, `email`, `department`, `salary`
  (a decimal string) and `currency`. `skills` is optional. `employee_id` may be an
  `int`, an integral `float64`, or a decimal string. A nil value, a float with a
  fraction, or any other type gives `ErrInvalidEmployee`. Extra keys are ignored.
- `NewEmployeeFromCSV` takes `id, name, email, department, salary, currency, skills`.
  Skills are separated by `;`.
- Skills are trimmed, lower-cased, de-duplicated (the first occurrence is kept), and
  empty entries are dropped. No skills give `nil`.
- `Skills()` returns a copy. `Equal` compares every field. The zero `Employee` is
  invalid: it has ID 0, no name, and no skills.

## Part 5 · Version: precedence, bumping and text encoding

`Version` is a semantic version with `uint64` components, an optional prerelease and
optional build metadata. Fields are unexported, so every value is valid.

```go
type Part int
const (PartMajor = iota + 1; PartMinor; PartPatch)

func NewVersion(major, minor, patch uint64, prerelease, build string) (Version, error)
func ParseVersion(text string) (Version, error)
func (v Version) Major() uint64
func (v Version) Minor() uint64
func (v Version) Patch() uint64
func (v Version) Prerelease() string
func (v Version) Build() string
func (v Version) Bump(p Part) (Version, error)
func (v Version) Compare(o Version) int
func (v Version) Equal(o Version) bool
func (v Version) String() string
func (v Version) MarshalText() ([]byte, error)
func (v *Version) UnmarshalText(text []byte) error
```

Behaviour and edge cases:

- `ParseVersion` accepts an optional `v` prefix, for example `v1.2.3-rc.1+build.5`.
  Core numbers must not have leading zeros. Components must fit in `uint64`.
  Malformed input gives `ErrInvalidVersion`.
- `NewVersion` checks that prerelease and build identifiers follow the SemVer rules.
  Numeric prerelease identifiers may not have leading zeros. Build metadata may.
- `Bump` increments the chosen part, resets lower parts, and clears prerelease and
  build. A zero or unknown `Part` gives `ErrInvalidPart`.
- `Compare` follows SemVer precedence. A release is greater than its prereleases.
  Numeric identifiers compare as numbers, and numeric identifiers rank below alphanumeric
  ones. The expected order is `1.0.0-alpha < 1.0.0-alpha.1 < 1.0.0-alpha.beta < 1.0.0-beta
  < 1.0.0-beta.2 < 1.0.0-beta.11 < 1.0.0-rc.1 < 1.0.0`. Build metadata is ignored.
- `Equal` follows `Compare`. The `==` operator compares build metadata too.
- `MarshalText` uses the canonical form without `v`. `UnmarshalText` leaves the
  receiver unchanged on error.
- The zero `Version` is `0.0.0`.

## Concepts exercised

- Structs as invariant-holding types: unexported fields plus a constructor.
- The zero value as a usable state (`Money{}`, `Cart{}`, `Version{}`), and when it should be
  an error.
- Value versus pointer receivers, and which methods need a pointer.
- Immutable values (`Product`, `Money`, `Version`) and copy-returning methods (`Restock`,
  `Skills`).
- Embedding for shared behaviour (`ledger`) and method overriding (`Withdraw`).
- Mutexes inside structs: why they are never copied, and why accounts are used through
  pointers.
- Interfaces satisfied by pointer types, and why a value type does not satisfy them.
- `fmt.Stringer`, `fmt.GoStringer`, `fmt.Formatter`, `encoding.TextMarshaler` and
  `encoding.TextUnmarshaler`.
- Comparability: `==` on structs, and why `Employee` needs `Equal`.
- Error wrapping with `%w` and sentinel errors checked with `errors.Is`.
- `iter.Seq2` for iterating a collection.
- Checked integer arithmetic, rounding and exact allocation.
- Concurrency with a lock held across check-and-update, tested with `sync.WaitGroup.Go`.

## Run

```sh
go test ./exercises/07_structs_and_methods/
go test ./exercises/07_structs_and_methods/ -run Part2 -v
go run ./tools/solutions ./exercises/07_structs_and_methods/
```

On Linux and macOS, also run the concurrency tests under the race detector:
`go test -race ./exercises/07_structs_and_methods/`. The race detector needs cgo, so
it does not run on Windows.

## Stretch ideas (untested)

- Make `Money` generic over a currency type parameter so that mixing currencies fails
  at compile time. Compare the design costs with the runtime check.
- Add a `Ledger` that records each operation as an immutable entry and rebuilds the
  balance by replaying them. Try `slices.Values` on the history.
- Replace the savings withdrawal limit with a pluggable policy interface and decide
  whether the policy belongs in the struct or in the caller.
- Add `Employee.MarshalJSON` and `UnmarshalJSON` that round-trip through
  `NewEmployeeFromMap`. Decide which one owns validation.
- Support full SemVer version ranges (`^1.2`, `~1.2.3`) with a `Range` type.
- Implement `sql.Scanner` and `driver.Valuer` for `Money` so that it can be stored as
  an integer column.
