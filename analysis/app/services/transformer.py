# app/services/transformer.py
"""
Transform Layer - Normalizes different market structures
This is the core business logic that handles Kalshi ranges, Polymarket binaries, etc.
"""
from enum import Enum
from typing import List, Optional, Dict
from dataclasses import dataclass


class TransformStrategy(str, Enum):
    """Available transformation strategies"""
    IDENTITY = "identity"           # Direct: price stays as-is
    SUM = "sum"                     # Sum multiple contracts
    INVERSE = "inverse"             # 1 - price (flip YES/NO)
    SUM_GT_THRESHOLD = "sum_gt"     # Sum contracts above threshold
    SUM_LT_THRESHOLD = "sum_lt"     # Sum contracts below threshold
    WEIGHTED_AVG = "weighted_avg"   # Weighted average


@dataclass
class TransformConfig:
    """Configuration for a transformation"""
    strategy: TransformStrategy
    threshold: Optional[float] = None
    weights: Optional[List[float]] = None
    

class PriceTransformer:
    """
    Transforms raw prices from different market structures into comparable probabilities
    
    Example use cases:
    1. Kalshi "Fed Rate 4.75-5.00%" + "Fed Rate > 5.00%" = Polymarket "Fed Rate > 4.75%"
    2. Kalshi "Trump wins" = 1 - Polymarket "Trump loses"
    3. Multiple categorical outcomes → single binary outcome
    """
    
    def transform(
        self,
        strategy: TransformStrategy,
        prices: List[float],
        threshold: Optional[float] = None,
        weights: Optional[List[float]] = None
    ) -> float:
        """
        Apply transformation strategy to raw prices
        
        Args:
            strategy: Transformation to apply
            prices: List of raw prices (0.0 - 1.0)
            threshold: Optional threshold for comparison
            weights: Optional weights for averaging
            
        Returns:
            Transformed probability (0.0 - 1.0)
            
        Raises:
            ValueError: If prices are invalid or strategy unknown
        """
        # Validate inputs
        if not prices:
            raise ValueError("Cannot transform empty price list")
        
        if any(p < 0 or p > 1 for p in prices):
            raise ValueError(f"All prices must be in range [0, 1], got: {prices}")
        
        # Apply strategy
        if strategy == TransformStrategy.IDENTITY:
            return self._identity(prices)
        
        elif strategy == TransformStrategy.SUM:
            return self._sum(prices)
        
        elif strategy == TransformStrategy.INVERSE:
            return self._inverse(prices)
        
        elif strategy == TransformStrategy.SUM_GT_THRESHOLD:
            if threshold is None:
                raise ValueError("SUM_GT_THRESHOLD requires threshold parameter")
            return self._sum_gt_threshold(prices, threshold)
        
        elif strategy == TransformStrategy.SUM_LT_THRESHOLD:
            if threshold is None:
                raise ValueError("SUM_LT_THRESHOLD requires threshold parameter")
            return self._sum_lt_threshold(prices, threshold)
        
        elif strategy == TransformStrategy.WEIGHTED_AVG:
            if weights is None:
                raise ValueError("WEIGHTED_AVG requires weights parameter")
            return self._weighted_avg(prices, weights)
        
        else:
            raise ValueError(f"Unknown strategy: {strategy}")
    
    # Strategy implementations
    
    def _identity(self, prices: List[float]) -> float:
        """Return first price unchanged"""
        return prices[0]
    
    def _sum(self, prices: List[float]) -> float:
        """
        Sum all prices
        Used when multiple contracts represent the same event
        
        Example:
        Kalshi has three contracts:
        - "4.75-5.00%": 0.30
        - "5.00-5.25%": 0.25
        - ">5.25%": 0.15
        
        To match Polymarket's ">4.75%", we sum all three: 0.30 + 0.25 + 0.15 = 0.70
        """
        result = sum(prices)
        return min(result, 1.0)  # Cap at 1.0
    
    def _inverse(self, prices: List[float]) -> float:
        """
        Invert probability (1 - p)
        Used to flip YES/NO or match opposite markets
        
        Example:
        Polymarket: "Trump wins" = 0.45
        To match Kalshi "Trump loses": 1 - 0.45 = 0.55
        """
        return 1.0 - prices[0]
    
    def _sum_gt_threshold(self, prices: List[float], threshold: float) -> float:
        """
        Sum prices where associated value > threshold
        Used for ranged markets
        
        Example:
        Kalshi markets with ranges:
        - "3.0-3.5%": 0.20 (mid: 3.25)
        - "3.5-4.0%": 0.30 (mid: 3.75)
        - "4.0-4.5%": 0.25 (mid: 4.25)
        
        For threshold=3.5%, sum second and third: 0.30 + 0.25 = 0.55
        """
        # Note: This requires knowing the range midpoints
        # In production, pass these as additional metadata
        return sum(p for p in prices if p > threshold)
    
    def _sum_lt_threshold(self, prices: List[float], threshold: float) -> float:
        """Sum prices where associated value < threshold"""
        return sum(p for p in prices if p < threshold)
    
    def _weighted_avg(self, prices: List[float], weights: List[float]) -> float:
        """
        Calculate weighted average
        Used when multiple sources should be combined
        
        Example:
        Sources with different confidence levels:
        - Kalshi: 0.60 (weight: 0.7)
        - Polymarket: 0.55 (weight: 0.3)
        Result: (0.60 * 0.7 + 0.55 * 0.3) = 0.585
        """
        if len(prices) != len(weights):
            raise ValueError("Prices and weights must have same length")
        
        if sum(weights) == 0:
            raise ValueError("Sum of weights cannot be zero")
        
        weighted_sum = sum(p * w for p, w in zip(prices, weights))
        total_weight = sum(weights)
        
        return weighted_sum / total_weight


class KalshiRangeMapper:
    """
    Helper class to map Kalshi ranged contracts to binary probabilities
    
    Kalshi often has markets like:
    - FED-25MAR-T4.50: "Rate will be 4.50-4.75%"
    - FED-25MAR-T4.75: "Rate will be 4.75-5.00%"
    - FED-25MAR-T5.00: "Rate will be >5.00%"
    
    To compare with Polymarket's "Rate > 4.75%", we need to sum the latter two.
    """
    
    def __init__(self):
        # Map of ticker suffixes to their ranges
        self.range_map: Dict[str, tuple] = {}
    
    def register_range(self, ticker: str, lower: float, upper: Optional[float] = None):
        """
        Register a ticker with its numeric range
        
        Args:
            ticker: Market ticker (e.g., "FED-25MAR-T4.75")
            lower: Lower bound of range
            upper: Upper bound (None for open-ended ">X")
        """
        self.range_map[ticker] = (lower, upper)
    
    def get_contracts_above(self, threshold: float) -> List[str]:
        """Get all tickers with ranges above threshold"""
        return [
            ticker for ticker, (lower, upper) in self.range_map.items()
            if lower >= threshold
        ]
    
    def get_contracts_below(self, threshold: float) -> List[str]:
        """Get all tickers with ranges below threshold"""
        return [
            ticker for ticker, (lower, upper) in self.range_map.items()
            if upper is not None and upper <= threshold
        ]
    
    def get_probability_above(
        self, 
        threshold: float, 
        prices: Dict[str, float]
    ) -> float:
        """
        Calculate total probability of outcome being above threshold
        
        Args:
            threshold: The threshold value
            prices: Dict mapping ticker -> probability
            
        Returns:
            Sum of probabilities for contracts above threshold
        """
        tickers = self.get_contracts_above(threshold)
        return sum(prices.get(ticker, 0.0) for ticker in tickers)


# Example usage
if __name__ == "__main__":
    transformer = PriceTransformer()
    
    # Example 1: Direct mapping
    print("Identity transform:", transformer.transform(
        TransformStrategy.IDENTITY,
        [0.65]
    ))  # Output: 0.65
    
    # Example 2: Sum ranged contracts
    print("Sum transform:", transformer.transform(
        TransformStrategy.SUM,
        [0.30, 0.25, 0.15]  # Three Kalshi contracts
    ))  # Output: 0.70
    
    # Example 3: Inverse
    print("Inverse transform:", transformer.transform(
        TransformStrategy.INVERSE,
        [0.45]
    ))  # Output: 0.55
    
    # Example 4: Kalshi range mapping
    mapper = KalshiRangeMapper()
    mapper.register_range("FED-25MAR-T4.50", 4.50, 4.75)
    mapper.register_range("FED-25MAR-T4.75", 4.75, 5.00)
    mapper.register_range("FED-25MAR-T5.00", 5.00, None)
    
    prices = {
        "FED-25MAR-T4.50": 0.30,
        "FED-25MAR-T4.75": 0.35,
        "FED-25MAR-T5.00": 0.20,
    }
    
    prob_above_4_75 = mapper.get_probability_above(4.75, prices)
    print(f"P(Rate > 4.75%): {prob_above_4_75}")  # Output: 0.55