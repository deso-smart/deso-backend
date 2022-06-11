package routes

import (
	"fmt"
	"github.com/deso-smart/deso-core/v2/lib"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	desoPubKeyBase58Check    = DESOCoinIdentifierString // represents $DESO
	daoCoinPubKeyBase58Check = "TestDAOCoinPubKey"      // represents valid DAO coin public key
)

func TestCalculateScaledExchangeRate(t *testing.T) {
	// equivalent to 1e9
	desoToDaoCoinBaseUnitsScalingFactor := getDESOToDAOCoinBaseUnitsScalingFactor()

	type testCaseType struct {
		floatValue                float64
		expectedWholeNumberDigits int64
		decimalDigitExponent      int64
	}

	// Convenience type to define exchange rates and expected uint256 scaled exchange rates.
	// Given a test case {100.1, 100, -1}, it means that our float exchangeRate is 100.1
	// and the expected uint256 scaled exchange rate is (1e38 * 100) + (1e38 / 10). This is an easy
	// way to test a sliding window of precision with both large and small numbers
	successTestCases := []testCaseType{
		{1.1, 1, -10},                                 // 2 digits
		{0.00000000000001, 0, -100000000000000},       // smallest supported number
		{1.0000000000001, 1, -10000000000000},         // 15 digits, no truncate
		{1000000000000.1, 1000000000000, -10},         // 15 digits, no truncate
		{10000000000000.01, 10000000000000, 0},        // 16 digits, which truncates everything after decimal point
		{100000000000001, 100000000000001, 0},         // 15 digits, no truncate
		{1000000000000001, 1000000000000000, 0},       // 16 digits, truncates everything below top 15 digits
		{1234567890123456789, 1234567890123450000, 0}, // 19 digits, truncates everything below top 15 digits
	}

	// Test when buying coin is a DAO coin and selling coin is a DAO coin, for various exchange rates
	for _, testCase := range successTestCases {
		exchangeRate := testCase.floatValue
		expectedScaledExchangeRate := uint256.NewInt()
		if testCase.expectedWholeNumberDigits > 0 {
			expectedScaledExchangeRate = uint256.NewInt().Mul(
				lib.OneE38, uint256.NewInt().SetUint64(uint64(testCase.expectedWholeNumberDigits)),
			)
		}

		if testCase.decimalDigitExponent < 0 {
			expectedScaledExchangeRate.Add(
				expectedScaledExchangeRate,
				uint256.NewInt().Div(lib.OneE38, uint256.NewInt().SetUint64(uint64(-testCase.decimalDigitExponent))),
			)
		}
		scaledExchangeRate, err := CalculateScaledExchangeRateFromFloat(
			daoCoinPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			exchangeRate,
		)
		require.NoError(t, err)
		require.Equal(t, expectedScaledExchangeRate, scaledExchangeRate)
	}

	// Test when buying coin is a DAO coin and selling coin is $DESO
	{
		scaledExchangeRate, err := CalculateScaledExchangeRateFromFloat(
			daoCoinPubKeyBase58Check,
			desoPubKeyBase58Check,
			1.0,
		)
		require.NoError(t, err)
		// expectedScaledExchangeRate / 1e9
		expectedScaledExchangeRate := uint256.NewInt().Div(lib.OneE38, desoToDaoCoinBaseUnitsScalingFactor)
		require.Equal(t, expectedScaledExchangeRate, scaledExchangeRate)
	}

	// Test when buying coin is $DESO and selling coin is DAO coin
	{
		scaledExchangeRate, err := CalculateScaledExchangeRateFromFloat(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			1.0,
		)
		require.NoError(t, err)
		expectedScaledExchangeRate := uint256.NewInt().Mul(
			lib.OneE38,
			desoToDaoCoinBaseUnitsScalingFactor,
		)
		require.Equal(t, expectedScaledExchangeRate, scaledExchangeRate)
	}

	failingTestCases := []float64{
		0.0000000000000001,                        // 1e-16 is too small
		10000000000000000000000000000000000000000, // 1e40 is too big
	}

	for _, exchangeRate := range failingTestCases {
		_, err := CalculateScaledExchangeRateFromFloat(
			daoCoinPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			exchangeRate,
		)
		require.Error(t, err)
	}
}

func TestCalculateScaledExchangeRateFromPriceString(t *testing.T) {
	type testCaseType struct {
		OperationType                               lib.DAOCoinLimitOrderOperationType
		Price                                       string
		ExpectedExchangeRateCoinsToSellPerCoinToBuy string
	}

	successTestCases := []testCaseType{
		{lib.DAOCoinLimitOrderOperationTypeBID, "1", "100000000000000000000000000000000000000"}, // 1 * 1e38
		{lib.DAOCoinLimitOrderOperationTypeASK, "1", "100000000000000000000000000000000000000"}, // 1e38 / 1

		// Integer price with decimal point
		{lib.DAOCoinLimitOrderOperationTypeBID, "1.0", "100000000000000000000000000000000000000"}, // 1 * 1e38
		{lib.DAOCoinLimitOrderOperationTypeASK, "1.0", "100000000000000000000000000000000000000"}, // 1e38 / 1

		{lib.DAOCoinLimitOrderOperationTypeBID, "20", "2000000000000000000000000000000000000000"}, // 20 * 1e38
		{lib.DAOCoinLimitOrderOperationTypeASK, "20", "5000000000000000000000000000000000000"},    // 1e38 / 20

		// Price with irrational calculated exchange rate
		{lib.DAOCoinLimitOrderOperationTypeBID, "3", "300000000000000000000000000000000000000"}, // 3 * 1e38
		{lib.DAOCoinLimitOrderOperationTypeASK, "3", "33333333333333333333333333333333333334"},  // ceil(1e38 / 3)

		// Price < 1
		{lib.DAOCoinLimitOrderOperationTypeBID, "0.005", "500000000000000000000000000000000000"},      // 0.005 * 1e38
		{lib.DAOCoinLimitOrderOperationTypeASK, "0.005", "20000000000000000000000000000000000000000"}, // 1e38 / 0.005

		// Decimal value with no whole number portion
		{lib.DAOCoinLimitOrderOperationTypeBID, ".005", "500000000000000000000000000000000000"},      // 0.005 * 1e38
		{lib.DAOCoinLimitOrderOperationTypeASK, ".005", "20000000000000000000000000000000000000000"}, // 1e38 / 0.005

		// Smallest possible price
		{lib.DAOCoinLimitOrderOperationTypeBID, "0.00000000000000000000000000000000000001", "1"}, // 1e-38 * 1e38
		{
			lib.DAOCoinLimitOrderOperationTypeASK,
			"0.00000000000000000000000000000000000001",
			"10000000000000000000000000000000000000000000000000000000000000000000000000000",
		}, // 1e38 * 1e38

		// An extremely large price (1e38)
		{
			lib.DAOCoinLimitOrderOperationTypeBID,
			"100000000000000000000000000000000000000",
			"10000000000000000000000000000000000000000000000000000000000000000000000000000", // 1e38 * 1e38
		},
		{
			lib.DAOCoinLimitOrderOperationTypeASK,
			"100000000000000000000000000000000000000",
			"1", // 1e-38 * 1e38
		},

		// Price digits under 1e-38 are truncated
		{lib.DAOCoinLimitOrderOperationTypeBID, "0.00000000000000000000000000000000000001234", "1"}, // 1e-38 * 1e38
	}

	// Test when buying coin is a DAO coin and selling coin is a DAO coin, for various exchange rates
	for _, testCase := range successTestCases {
		scaledExchangeRate, err := CalculateScaledExchangeRateFromPriceString(
			daoCoinPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			testCase.Price,
			testCase.OperationType,
		)

		require.NoError(t, err)
		require.Equal(t, testCase.ExpectedExchangeRateCoinsToSellPerCoinToBuy, fmt.Sprintf("%v", scaledExchangeRate))
	}

	errorTestPrices := []string{
		"0.000000000000000000000000000000000000001", // 1e-39 is too small
		"10000000000000000000000000000000000000000", // 1e40 is too big
		"0",
		"0.0",
		"-1",
		"-1.0",
		"-.1",
		"a",
		"2.a",
		"a.2",
		"",
	}

	// Test when buying coin is a DAO coin and selling coin is a DAO coin, for various exchange rates
	for _, price := range errorTestPrices {
		_, err := CalculateScaledExchangeRateFromPriceString(
			daoCoinPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			price,
			lib.DAOCoinLimitOrderOperationTypeASK,
		)
		require.Error(t, err)

		_, err = CalculateScaledExchangeRateFromPriceString(
			daoCoinPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			price,
			lib.DAOCoinLimitOrderOperationTypeBID,
		)
		require.Error(t, err)
	}
}

func TestCalculateExchangeRateAsFloat(t *testing.T) {
	desoToDaoCoinBaseUnitsScalingFactor := getDESOToDAOCoinBaseUnitsScalingFactor()

	// equivalent to 100.00000001
	scaledExchangeRate := uint256.NewInt().Add(
		uint256.NewInt().Mul(lib.OneE38, uint256.NewInt().SetUint64(100)),       // 100
		uint256.NewInt().Div(lib.OneE38, uint256.NewInt().SetUint64(100000000)), // 0.00000001
	)
	expectedExchangeRate := 100.00000001

	// Test when buying coin is a DAO coin and selling coin is a DAO coin order
	{
		scaledValue, err := CalculateFloatFromScaledExchangeRate(
			daoCoinPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			scaledExchangeRate,
		)
		require.NoError(t, err)
		require.Equal(t, expectedExchangeRate, scaledValue)
	}

	// Test when buying coin is a DAO coin and selling coin is $DESO
	{
		exchangeRate, err := CalculateFloatFromScaledExchangeRate(
			daoCoinPubKeyBase58Check,
			desoPubKeyBase58Check,
			scaledExchangeRate,
		)
		require.NoError(t, err)
		expectedReScaledExchangeRate := expectedExchangeRate * float64(desoToDaoCoinBaseUnitsScalingFactor.Uint64())
		require.Equal(t, expectedReScaledExchangeRate, exchangeRate)
	}

	// Test when buying coin is $DESO coin and buying coin is $DESO
	{
		exchangeRate, err := CalculateFloatFromScaledExchangeRate(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			scaledExchangeRate,
		)
		require.NoError(t, err)
		expectedReScaledExchangeRate := expectedExchangeRate / float64(desoToDaoCoinBaseUnitsScalingFactor.Uint64())
		require.Equal(t, expectedReScaledExchangeRate, exchangeRate)
	}
}

func TestCalculatePriceStringFromScaledExchangeRate(t *testing.T) {
	desoToDaoCoinBaseUnitsScalingFactor := getDESOToDAOCoinBaseUnitsScalingFactor()

	// equivalent to 100 scaled up by 1e38
	scaledExchangeRate := uint256.NewInt().Mul(lib.OneE38, uint256.NewInt().SetUint64(100))

	expectedStringExchangeRate := "100.0"
	expectedInvertedStringExchangeRate := "0.01"

	// Test when buying coin is a DAO coin, selling coin is a DAO coin order, and operation type is BID
	{
		priceString, err := CalculatePriceStringFromScaledExchangeRate(
			daoCoinPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			scaledExchangeRate,
			DAOCoinLimitOrderOperationTypeStringBID,
		)
		require.NoError(t, err)
		require.Equal(t, expectedStringExchangeRate, priceString)
	}

	// Test when buying coin is a DAO coin, selling coin is a DAO coin order, and operation type is ASK
	{
		priceString, err := CalculatePriceStringFromScaledExchangeRate(
			daoCoinPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			scaledExchangeRate,
			DAOCoinLimitOrderOperationTypeStringASK,
		)
		require.NoError(t, err)
		require.Equal(t, expectedInvertedStringExchangeRate, priceString)
	}

	// Test when buying coin is a DAO coin, selling coin is $DESO, and operation type is BID
	{
		exchangeRate, err := CalculatePriceStringFromScaledExchangeRate(
			daoCoinPubKeyBase58Check,
			desoPubKeyBase58Check,
			// need to account for exchange rate being scaled up by 1e9 for orders selling deso for dao coins
			uint256.NewInt().Div(scaledExchangeRate, desoToDaoCoinBaseUnitsScalingFactor),
			DAOCoinLimitOrderOperationTypeStringBID,
		)
		require.NoError(t, err)
		require.Equal(t, expectedStringExchangeRate, exchangeRate)
	}

	// Test when buying coin is a DAO coin, selling coin is $DESO, and operation type is ASK
	{
		exchangeRate, err := CalculatePriceStringFromScaledExchangeRate(
			daoCoinPubKeyBase58Check,
			desoPubKeyBase58Check,
			// need to account for exchange rate being scaled up by 1e9 for orders selling deso for dao coins
			uint256.NewInt().Div(scaledExchangeRate, desoToDaoCoinBaseUnitsScalingFactor),
			DAOCoinLimitOrderOperationTypeStringASK,
		)
		require.NoError(t, err)
		require.Equal(t, expectedInvertedStringExchangeRate, exchangeRate)
	}

	// Test when buying coin is $DESO coin, buying coin is $DESO, and operation type is BID
	{
		exchangeRate, err := CalculatePriceStringFromScaledExchangeRate(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			// need to account for exchange rate being scaled down by 1e9 for orders selling dao coins for deso
			uint256.NewInt().Mul(scaledExchangeRate, desoToDaoCoinBaseUnitsScalingFactor),
			DAOCoinLimitOrderOperationTypeStringBID,
		)
		require.NoError(t, err)
		require.Equal(t, expectedStringExchangeRate, exchangeRate)
	}

	// Test when buying coin is $DESO coin, buying coin is $DESO, and operation type is ASK
	{
		exchangeRate, err := CalculatePriceStringFromScaledExchangeRate(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			// need to account for exchange rate being scaled down by 1e9 for orders selling dao coins for deso
			uint256.NewInt().Mul(scaledExchangeRate, desoToDaoCoinBaseUnitsScalingFactor),
			DAOCoinLimitOrderOperationTypeStringASK,
		)
		require.NoError(t, err)
		require.Equal(t, expectedInvertedStringExchangeRate, exchangeRate)
	}
}

func TestCalculateQuantityToFillAsBaseUnits(t *testing.T) {
	expectedValueIfDESO := uint256.NewInt().SetUint64(lib.NanosPerUnit)
	expectedValueIfDAOCoin := &(*lib.BaseUnitsPerCoin)

	quantity := float64(1)

	// Bid order to buy $DESO using a DAO coin
	{
		scaledQuantity, err := CalculateQuantityToFillAsBaseUnits(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringBID,
			formatFloatAsString(quantity),
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDESO, scaledQuantity)
	}

	// Bid order to buy a DAO coin using $DESO
	{
		scaledQuantity, err := CalculateQuantityToFillAsBaseUnits(
			daoCoinPubKeyBase58Check,
			desoPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringBID,
			formatFloatAsString(quantity),
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDAOCoin, scaledQuantity)
	}

	// Ask order to sell $DESO for a DAO coin
	{
		scaledQuantity, err := CalculateQuantityToFillAsBaseUnits(
			daoCoinPubKeyBase58Check,
			desoPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringASK,
			formatFloatAsString(quantity),
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDESO, scaledQuantity)
	}

	// Ask order to sell a DAO coin for $DESO
	{
		scaledQuantity, err := CalculateQuantityToFillAsBaseUnits(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringASK,
			formatFloatAsString(quantity),
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDAOCoin, scaledQuantity)
	}

	failingTestCaseQuantities := []string{
		"0", "0.0", ".0", "-1", "-1.1", "-.1", "a", "a.b", ".a",
	}

	for _, testCaseQuantity := range failingTestCaseQuantities {
		// BID order
		{
			_, err := CalculateQuantityToFillAsBaseUnits(
				daoCoinPubKeyBase58Check,
				daoCoinPubKeyBase58Check,
				DAOCoinLimitOrderOperationTypeStringBID,
				testCaseQuantity,
			)
			require.Error(t, err)
		}

		// Ask order
		{
			_, err := CalculateQuantityToFillAsBaseUnits(
				daoCoinPubKeyBase58Check,
				daoCoinPubKeyBase58Check,
				DAOCoinLimitOrderOperationTypeStringASK,
				testCaseQuantity,
			)
			require.Error(t, err)
		}
	}
}

func TestCalculateQuantityToFillAsFloat(t *testing.T) {
	scaledQuantity := lib.BaseUnitsPerCoin
	expectedValueIfDESO := float64(getDESOToDAOCoinBaseUnitsScalingFactor().Uint64()) // 1e9
	expectedValueIfDAOCoin := float64(1)

	// Bid order to buy $DESO using a DAO coin
	{
		quantity, err := CalculateFloatQuantityFromBaseUnits(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringBID,
			scaledQuantity,
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDESO, quantity)
	}

	// Bid order to buy a DAO coin using $DESO
	{
		quantity, err := CalculateFloatQuantityFromBaseUnits(
			daoCoinPubKeyBase58Check,
			desoPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringBID,
			scaledQuantity,
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDAOCoin, quantity)
	}

	// Ask order to sell $DESO for a DAO coin
	{
		quantity, err := CalculateFloatQuantityFromBaseUnits(
			daoCoinPubKeyBase58Check,
			desoPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringASK,
			scaledQuantity,
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDESO, quantity)
	}

	// Ask order to sell a DAO coin for $DESO
	{
		quantity, err := CalculateFloatQuantityFromBaseUnits(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringASK,
			scaledQuantity,
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDAOCoin, quantity)
	}
}

func TestCalculateStringQuantityFromBaseUnits(t *testing.T) {
	scaledQuantity := lib.BaseUnitsPerCoin
	expectedValueIfDESO := "1000000000.0" // 1e9
	expectedValueIfDAOCoin := "1.0"

	// Bid order to buy $DESO using a DAO coin
	{
		quantity, err := CalculateStringQuantityFromBaseUnits(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringBID,
			scaledQuantity,
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDESO, quantity)
	}

	// Bid order to buy a DAO coin using $DESO
	{
		quantity, err := CalculateStringQuantityFromBaseUnits(
			daoCoinPubKeyBase58Check,
			desoPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringBID,
			scaledQuantity,
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDAOCoin, quantity)
	}

	// Ask order to sell $DESO for a DAO coin
	{
		quantity, err := CalculateStringQuantityFromBaseUnits(
			daoCoinPubKeyBase58Check,
			desoPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringASK,
			scaledQuantity,
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDESO, quantity)
	}

	// Ask order to sell a DAO coin for $DESO
	{
		quantity, err := CalculateStringQuantityFromBaseUnits(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringASK,
			scaledQuantity,
		)
		require.NoError(t, err)
		require.Equal(t, expectedValueIfDAOCoin, quantity)
	}

	// zero quantity for BID order
	{
		_, err := CalculateStringQuantityFromBaseUnits(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringBID,
			uint256.NewInt().SetUint64(0),
		)
		require.Error(t, err)
	}

	// zero quantity fpr ASK order
	{
		_, err := CalculateStringQuantityFromBaseUnits(
			desoPubKeyBase58Check,
			daoCoinPubKeyBase58Check,
			DAOCoinLimitOrderOperationTypeStringASK,
			uint256.NewInt().SetUint64(0),
		)
		require.Error(t, err)
	}
}
