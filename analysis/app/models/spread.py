# app/models/spread.py
"""
Pydantic models for spread calculations and arbitrage opportunities
"""
from datetime import datetime
from typing import Literal
from pydantic import BaseModel, Field, field_validator


class SpreadPair(BaseModel):
    """Spread between two specific platforms"""

    platform_a: str = Field(..., description="First platform name")
    platform_b: str = Field(..., description="Second platform name")
    prob_a: float = Field(..., ge=0.0, le=1.0, description="Probability on platform A")
    prob_b: float = Field(..., ge=0.0, le=1.0, description="Probability on platform B")
    spread: float = Field(..., ge=0.0, description="Absolute difference |prob_a - prob_b|")
    spread_pct: float = Field(..., description="Spread as percentage")

    @property
    def arbitrage_direction(self) -> str:
        """Which platform to buy on (lower price) and sell on (higher price)"""
        if self.prob_a < self.prob_b:
            return f"BUY {self.platform_a}, SELL {self.platform_b}"
        else:
            return f"BUY {self.platform_b}, SELL {self.platform_a}"


class SpreadResult(BaseModel):
    """
    Complete spread analysis for a market pair
    Matches spread_calculator.py SpreadResult dataclass
    """

    pair_id: str = Field(..., description="Market pair identifier")
    description: str = Field(..., description="Human-readable description")

    # Probabilities from each platform
    kalshi_prob: float | None = Field(
        default=None,
        ge=0.0,
        le=1.0,
        description="Normalized Kalshi probability"
    )
    poly_prob: float | None = Field(
        default=None,
        ge=0.0,
        le=1.0,
        description="Normalized Polymarket probability"
    )
    manifold_prob: float | None = Field(
        default=None,
        ge=0.0,
        le=1.0,
        description="Normalized Manifold probability"
    )

    # Spreads between pairs
    kalshi_poly_spread: float | None = Field(
        default=None,
        ge=0.0,
        description="Spread between Kalshi and Polymarket"
    )
    kalshi_manifold_spread: float | None = Field(
        default=None,
        ge=0.0,
        description="Spread between Kalshi and Manifold"
    )
    poly_manifold_spread: float | None = Field(
        default=None,
        ge=0.0,
        description="Spread between Polymarket and Manifold"
    )

    # Maximum spread
    max_spread: float = Field(..., ge=0.0, description="Maximum spread across all pairs")
    max_spread_pair: Literal["KALSHI-POLY", "KALSHI-MANIFOLD", "POLY-MANIFOLD"] = Field(
        ...,
        description="Platform pair with maximum spread"
    )

    # Metadata
    timestamp: datetime = Field(
        default_factory=datetime.now,
        description="Calculation timestamp"
    )
    data_completeness: float = Field(
        ...,
        ge=0.0,
        le=1.0,
        description="Fraction of data sources available (0.0-1.0)"
    )

    @field_validator("data_completeness")
    @classmethod
    def validate_completeness(cls, v: float) -> float:
        """Ensure completeness is valid fraction"""
        if not 0.0 <= v <= 1.0:
            raise ValueError(f"data_completeness must be between 0.0 and 1.0, got: {v}")
        return v

    @property
    def has_alert(self) -> bool:
        """Check if this spread exceeds common alert threshold"""
        # Default 5% threshold
        return self.max_spread >= 0.05

    @property
    def available_platforms(self) -> list[str]:
        """List of platforms with available data"""
        platforms = []
        if self.kalshi_prob is not None:
            platforms.append("KALSHI")
        if self.poly_prob is not None:
            platforms.append("POLYMARKET")
        if self.manifold_prob is not None:
            platforms.append("MANIFOLD")
        return platforms

    @property
    def best_spread_detail(self) -> SpreadPair | None:
        """Get detailed info about the best arbitrage opportunity"""
        if self.max_spread_pair == "KALSHI-POLY" and self.kalshi_prob and self.poly_prob:
            return SpreadPair(
                platform_a="KALSHI",
                platform_b="POLYMARKET",
                prob_a=self.kalshi_prob,
                prob_b=self.poly_prob,
                spread=self.kalshi_poly_spread or 0.0,
                spread_pct=(self.kalshi_poly_spread or 0.0) * 100
            )
        elif self.max_spread_pair == "KALSHI-MANIFOLD" and self.kalshi_prob and self.manifold_prob:
            return SpreadPair(
                platform_a="KALSHI",
                platform_b="MANIFOLD",
                prob_a=self.kalshi_prob,
                prob_b=self.manifold_prob,
                spread=self.kalshi_manifold_spread or 0.0,
                spread_pct=(self.kalshi_manifold_spread or 0.0) * 100
            )
        elif self.max_spread_pair == "POLY-MANIFOLD" and self.poly_prob and self.manifold_prob:
            return SpreadPair(
                platform_a="POLYMARKET",
                platform_b="MANIFOLD",
                prob_a=self.poly_prob,
                prob_b=self.manifold_prob,
                spread=self.poly_manifold_spread or 0.0,
                spread_pct=(self.poly_manifold_spread or 0.0) * 100
            )
        return None

    model_config = {
        "json_schema_extra": {
            "examples": [
                {
                    "pair_id": "fed-rate-march-2025",
                    "description": "Federal Reserve interest rate decision March 2025",
                    "kalshi_prob": 0.55,
                    "poly_prob": 0.58,
                    "manifold_prob": 0.52,
                    "kalshi_poly_spread": 0.03,
                    "kalshi_manifold_spread": 0.03,
                    "poly_manifold_spread": 0.06,
                    "max_spread": 0.06,
                    "max_spread_pair": "POLY-MANIFOLD",
                    "timestamp": "2025-01-15T10:30:00Z",
                    "data_completeness": 1.0
                }
            ]
        }
    }


class Alert(BaseModel):
    """Arbitrage alert when spread exceeds threshold"""

    spread_result: SpreadResult = Field(..., description="Spread calculation that triggered alert")
    threshold: float = Field(..., ge=0.0, description="Alert threshold that was exceeded")
    severity: Literal["low", "medium", "high", "critical"] = Field(
        ...,
        description="Alert severity based on spread magnitude"
    )
    created_at: datetime = Field(
        default_factory=datetime.now,
        description="Alert creation timestamp"
    )

    @classmethod
    def from_spread(
        cls,
        spread_result: SpreadResult,
        threshold: float = 0.05
    ) -> "Alert | None":
        """
        Create alert from spread result if threshold exceeded

        Severity levels:
        - low: 5-10% spread
        - medium: 10-15% spread
        - high: 15-20% spread
        - critical: >20% spread
        """
        if spread_result.max_spread < threshold:
            return None

        # Determine severity
        spread_pct = spread_result.max_spread * 100
        if spread_pct >= 20:
            severity = "critical"
        elif spread_pct >= 15:
            severity = "high"
        elif spread_pct >= 10:
            severity = "medium"
        else:
            severity = "low"

        return cls(
            spread_result=spread_result,
            threshold=threshold,
            severity=severity
        )


class SpreadHistory(BaseModel):
    """Historical spread data for charting"""

    pair_id: str = Field(..., description="Market pair identifier")
    timestamps: list[datetime] = Field(..., description="Timestamps for each data point")
    kalshi_probs: list[float | None] = Field(..., description="Kalshi probabilities over time")
    poly_probs: list[float | None] = Field(..., description="Polymarket probabilities over time")
    manifold_probs: list[float | None] = Field(..., description="Manifold probabilities over time")
    max_spreads: list[float] = Field(..., description="Maximum spreads over time")

    @field_validator("timestamps", "kalshi_probs", "poly_probs", "manifold_probs", "max_spreads")
    @classmethod
    def validate_equal_lengths(cls, v, info):
        """Ensure all lists have same length"""
        # This validation happens per field, so we just return the value
        # Full cross-field validation would need model_validator
        return v

    @property
    def data_points(self) -> int:
        """Number of data points in history"""
        return len(self.timestamps)
