# app/services/spread_calculator.py
"""
Spread Calculator - Calculates arbitrage opportunities between platforms
"""
from typing import Dict, List, Optional
from dataclasses import dataclass, asdict
from datetime import datetime
import asyncio

from app.services.transformer import PriceTransformer, TransformStrategy


@dataclass
class MarketPairConfig:
    """Configuration for a linked market pair"""
    id: str
    description: str
    
    # Kalshi configuration
    kalshi_tickers: List[str]
    kalshi_transform: TransformStrategy = TransformStrategy.IDENTITY
    kalshi_threshold: Optional[float] = None
    
    # Polymarket configuration
    poly_token_id: Optional[str] = None
    poly_transform: TransformStrategy = TransformStrategy.IDENTITY
    
    # Manifold configuration
    manifold_slug: Optional[str] = None
    manifold_transform: TransformStrategy = TransformStrategy.IDENTITY
    
    # Alert configuration
    alert_threshold: float = 0.05  # Alert when spread > 5%


@dataclass
class SpreadResult:
    """Result of spread calculation"""
    pair_id: str
    description: str
    
    # Probabilities
    kalshi_prob: Optional[float]
    poly_prob: Optional[float]
    manifold_prob: Optional[float]
    
    # Spread calculations
    kalshi_poly_spread: Optional[float]  # |kalshi - poly|
    kalshi_manifold_spread: Optional[float]
    poly_manifold_spread: Optional[float]
    
    # Maximum spread across all pairs
    max_spread: float
    max_spread_pair: str  # e.g., "KALSHI-POLY"
    
    # Metadata
    timestamp: datetime
    data_completeness: float  # Percentage of data available (0.0-1.0)
    
    def to_dict(self):
        return asdict(self)


class SpreadCalculator:
    """
    Calculates arbitrage opportunities between platforms
    
    Uses the Transform Layer to normalize different market structures,
    then calculates spreads between normalized probabilities.
    """
    
    def __init__(self):
        self.transformer = PriceTransformer()
        self.price_cache: Dict[str, Dict[str, float]] = {
            "KALSHI": {},
            "POLYMARKET": {}
        }
        self.last_update: Dict[str, datetime] = {}
    
    def update_price(self, source: str, contract_id: str, price: float):
        """
        Update cached price for a contract
        
        Args:
            source: "KALSHI", "POLYMARKET", or "MANIFOLD"
            contract_id: Contract/ticker/slug identifier
            price: Probability (0.0 - 1.0)
        """
        if source not in self.price_cache:
            raise ValueError(f"Unknown source: {source}")
        
        self.price_cache[source][contract_id] = price
        self.last_update[f"{source}:{contract_id}"] = datetime.now()
    
    def calculate_spread(self, pair_config: MarketPairConfig) -> Optional[SpreadResult]:
        """
        Calculate spread for a market pair
        
        Returns None if insufficient data available
        """
        # Get normalized probabilities from each source (Kalshi + Polymarket only)
        kalshi_prob = self._get_kalshi_probability(pair_config)
        poly_prob = self._get_poly_probability(pair_config)

        # Calculate data completeness (only 2 sources now)
        available_sources = sum([
            kalshi_prob is not None,
            poly_prob is not None,
        ])

        if available_sources < 2:
            return None  # Need both sources to calculate spread

        data_completeness = available_sources / 2.0

        # Calculate spread between Kalshi and Polymarket
        kalshi_poly_spread = None
        if kalshi_prob is not None and poly_prob is not None:
            kalshi_poly_spread = abs(kalshi_prob - poly_prob)

        # Only one spread pair now
        max_spread = kalshi_poly_spread if kalshi_poly_spread is not None else 0.0
        max_spread_pair = "KALSHI-POLY"

        return SpreadResult(
            pair_id=pair_config.id,
            description=pair_config.description,
            kalshi_prob=kalshi_prob,
            poly_prob=poly_prob,
            manifold_prob=None,  # Not used anymore
            kalshi_poly_spread=kalshi_poly_spread,
            kalshi_manifold_spread=None,  # Not used anymore
            poly_manifold_spread=None,  # Not used anymore
            max_spread=max_spread,
            max_spread_pair=max_spread_pair,
            timestamp=datetime.now(),
            data_completeness=data_completeness
        )
    
    def _get_kalshi_probability(self, config: MarketPairConfig) -> Optional[float]:
        """Get normalized Kalshi probability"""
        if not config.kalshi_tickers:
            return None
        
        # Collect prices for all configured tickers
        prices = []
        for ticker in config.kalshi_tickers:
            if ticker in self.price_cache["KALSHI"]:
                prices.append(self.price_cache["KALSHI"][ticker])
        
        if not prices:
            return None
        
        # If we don't have all tickers, might be incomplete
        if len(prices) < len(config.kalshi_tickers):
            # Could choose to return None or use partial data
            # For now, use what we have
            pass
        
        # Apply transformation
        try:
            return self.transformer.transform(
                config.kalshi_transform,
                prices,
                threshold=config.kalshi_threshold
            )
        except ValueError:
            return None
    
    def _get_poly_probability(self, config: MarketPairConfig) -> Optional[float]:
        """Get normalized Polymarket probability"""
        if not config.poly_token_id:
            return None
        
        if config.poly_token_id not in self.price_cache["POLYMARKET"]:
            return None
        
        price = self.price_cache["POLYMARKET"][config.poly_token_id]
        
        try:
            return self.transformer.transform(
                config.poly_transform,
                [price]
            )
        except ValueError:
            return None
    
    def _get_manifold_probability(self, config: MarketPairConfig) -> Optional[float]:
        """Get normalized Manifold probability"""
        if not config.manifold_slug:
            return None
        
        if config.manifold_slug not in self.price_cache["MANIFOLD"]:
            return None
        
        price = self.price_cache["MANIFOLD"][config.manifold_slug]
        
        try:
            return self.transformer.transform(
                config.manifold_transform,
                [price]
            )
        except ValueError:
            return None
    
    def calculate_all_spreads(
        self, 
        configs: List[MarketPairConfig]
    ) -> List[SpreadResult]:
        """Calculate spreads for all configured pairs"""
        results = []
        
        for config in configs:
            spread = self.calculate_spread(config)
            if spread is not None:
                results.append(spread)
        
        return results
    
    def get_alerts(
        self, 
        configs: List[MarketPairConfig]
    ) -> List[SpreadResult]:
        """
        Get spreads that exceed alert thresholds
        
        Returns list of SpreadResults where max_spread > alert_threshold
        """
        all_spreads = self.calculate_all_spreads(configs)
        
        alerts = []
        for spread in all_spreads:
            # Find the config to get alert threshold
            config = next((c for c in configs if c.id == spread.pair_id), None)
            if config and spread.max_spread > config.alert_threshold:
                alerts.append(spread)
        
        return alerts
    
    def get_cache_stats(self) -> Dict:
        """Get statistics about cached prices"""
        return {
            "kalshi_contracts": len(self.price_cache["KALSHI"]),
            "polymarket_contracts": len(self.price_cache["POLYMARKET"]),
            "total_contracts": sum(len(cache) for cache in self.price_cache.values()),
            "last_updates": len(self.last_update)
        }


# Example usage
if __name__ == "__main__":
    # Create calculator
    calc = SpreadCalculator()
    
    # Simulate price updates
    calc.update_price("KALSHI", "FED-25MAR-T4.75", 0.35)
    calc.update_price("KALSHI", "FED-25MAR-T5.00", 0.20)
    calc.update_price("POLYMARKET", "0x123abc", 0.58)
    calc.update_price("MANIFOLD", "fed-rate-march", 0.52)
    
    # Create config
    config = MarketPairConfig(
        id="fed-rate-march",
        description="Fed rate decision March 2025",
        kalshi_tickers=["FED-25MAR-T4.75", "FED-25MAR-T5.00"],
        kalshi_transform=TransformStrategy.SUM,  # Sum both contracts
        poly_token_id="0x123abc",
        manifold_slug="fed-rate-march",
        alert_threshold=0.05
    )
    
    # Calculate spread
    result = calc.calculate_spread(config)
    
    if result:
        print(f"Spread Result for {result.pair_id}:")
        print(f"  Kalshi: {result.kalshi_prob:.2%}")
        print(f"  Polymarket: {result.poly_prob:.2%}")
        print(f"  Manifold: {result.manifold_prob:.2%}")
        print(f"  Max Spread: {result.max_spread:.2%} ({result.max_spread_pair})")
        print(f"  Data Completeness: {result.data_completeness:.0%}")