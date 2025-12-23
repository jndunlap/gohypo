import os, json, requests
from openai import OpenAI
from dataclasses import dataclass
from typing import List, Dict, Optional, Tuple

@dataclass
class MarketData:
    ticker: str; title: str; category: str; tags: List[str]
    yes_bid: float; yes_ask: float; no_bid: float; no_ask: float
    volume: int; volume_24h: int; open_interest: int; last_price: float

@dataclass
class ArbitrageOpportunity:
    description: str; markets: List[MarketData]; potential_profit: float; risk_level: str; confidence_score: float

class KalshiAPIClient:
    BASE_URL = "https://api.elections.kalshi.com/trade-api/v2"
    def __init__(self, api_key=None):
        self.api_key = api_key or os.getenv("KALSHI_API_KEY")
        self.session = requests.Session()
        if self.api_key: self.session.headers.update({"Authorization": f"Bearer {self.api_key}", "Content-Type": "application/json"})

    def get_markets(self, limit=100, cursor=None, series_ticker=None, status="active"):
        url = f"{self.BASE_URL}/markets"
        params = {"limit": limit, "status": status}
        if cursor: params["cursor"] = cursor
        if series_ticker: params["series_ticker"] = series_ticker
        response = self.session.get(url, params=params)
        response.raise_for_status()
        return response.json()

    def get_series(self, ticker=None):
        url = f"{self.BASE_URL}/series/{ticker}" if ticker else f"{self.BASE_URL}/series"
        response = self.session.get(url)
        response.raise_for_status()
        return response.json()

class OpenAITagGenerator:
    def __init__(self, api_key=None):
        self.api_key = api_key or os.getenv("OPENAI_API_KEY")
        if not self.api_key: raise ValueError("OpenAI API key required")
        self.client = OpenAI(api_key=self.api_key)

    def generate_arbitrage_tags(self, context="", num_tags=10):
        prompt = f"Generate {num_tags} relevant tags for finding arbitrage opportunities in election prediction markets. {f'Context: {context}' if context else ''} Return only JSON array."
        response = self.client.chat.completions.create(model="gpt-4o-mini", messages=[{"role": "user", "content": prompt}], temperature=0.7, max_tokens=500)
        result = response.choices[0].message.content.strip()
        try: return json.loads(result)
        except: return [line.strip('"') for line in result.split('\n') if '"' in line][:num_tags]

class ArbitrageDetector:
    def __init__(self, kalshi_client, tag_generator):
        self.kalshi_client = kalshi_client
        self.tag_generator = tag_generator

    def find_correlated_markets(self, tags=None, min_correlation_threshold=0.6):
        markets_data = self.kalshi_client.get_markets(limit=500)
        markets = [MarketData(ticker=m["ticker"], title=m["title"], category=m["category"], tags=m.get("tags", []),
                             yes_bid=m.get("yes_bid", 0), yes_ask=m.get("yes_ask", 0), no_bid=m.get("no_bid", 0), no_ask=m.get("no_ask", 0),
                             volume=m.get("volume", 0), volume_24h=m.get("volume_24h", 0), open_interest=m.get("open_interest", 0), last_price=m.get("last_price", 0))
                  for m in markets_data.get("markets", [])]

        if tags:
            markets = [m for m in markets if any(tag.lower() in ' '.join(m.tags + [m.title, m.category]).lower() for tag in tags)]

        series_groups = {}
        for market in markets:
            series = market.ticker.split('-')[0] if '-' in market.ticker else market.category
            series_groups.setdefault(series, []).append(market)

        correlated_pairs = []
        for series_markets in series_groups.values():
            if len(series_markets) < 2: continue
            for i, m1 in enumerate(series_markets):
                for m2 in series_markets[i+1:]:
                    score = self._calculate_correlation_score(m1, m2)
                    if score >= min_correlation_threshold: correlated_pairs.append((m1, m2, score))

        return correlated_pairs

    def _calculate_correlation_score(self, m1, m2):
        score = 0
        tag_overlap = len(set(m1.tags) & set(m2.tags))
        total_tags = len(set(m1.tags) | set(m2.tags))
        if total_tags: score += (tag_overlap / total_tags) * 0.3

        title_words1, title_words2 = set(m1.title.lower().split()), set(m2.title.lower().split())
        title_overlap = len(title_words1 & title_words2)
        title_union = len(title_words1 | title_words2)
        if title_union: score += (title_overlap / title_union) * 0.4

        if m1.category == m2.category: score += 0.2
        series1 = m1.ticker.split('-')[0] if '-' in m1.ticker else m1.category
        series2 = m2.ticker.split('-')[0] if '-' in m2.ticker else m2.category
        if series1 == series2: score += 0.1
        return min(score, 1.0)

    def detect_arbitrage_opportunities(self, correlated_pairs):
        opportunities = []
        for m1, m2, correlation_score in correlated_pairs:
            price1, price2 = m1.last_price, m2.last_price
            if price1 + price2 < 0.95:
                profit = (1 - (price1 + price2)) * 100
                avg_volume = (m1.volume_24h + m2.volume_24h) / 2
                risk = "Low" if avg_volume > 10000 else "Medium" if avg_volume > 1000 else "High"
                opportunities.append(ArbitrageOpportunity(f"Arbitrage between '{m1.title}' and '{m2.title}'", [m1, m2], profit, risk, min(correlation_score, 0.9)))
        return sorted(opportunities, key=lambda x: x.potential_profit, reverse=True)

class ArbitrageFinder:
    def __init__(self, openai_key=None, kalshi_key=None):
        self.kalshi_client = KalshiAPIClient(kalshi_key)
        self.tag_generator = OpenAITagGenerator(openai_key)
        self.detector = ArbitrageDetector(self.kalshi_client, self.tag_generator)

    def find_opportunities(self, context="", num_tags=10, max_opportunities=10, use_series_data=True, correlation_threshold=0.6):
        if use_series_data: self.kalshi_client.get_series()
        tags = self.tag_generator.generate_arbitrage_tags(context, num_tags) or ["election", "politics", "democrat", "republican", "2024", "president"]
        correlated_pairs = self.detector.find_correlated_markets(tags, correlation_threshold)
        opportunities = self.detector.detect_arbitrage_opportunities(correlated_pairs)
        return opportunities[:max_opportunities]

    def print_opportunities(self, opportunities):
        if not opportunities: print("No arbitrage opportunities found."); return
        print("ARBITRAGE OPPORTUNITIES FOUND")
        print("="*50)
        for i, opp in enumerate(opportunities, 1):
            print(f"{i}. {opp.description}")
            print(f"   Profit: ${opp.potential_profit:.2f}")
            print(f"   Risk: {opp.risk_level} | Confidence: {opp.confidence_score:.2f}")
            for market in opp.markets:
                print(f"   • {market.ticker}: {market.title}")
                print(f"     Price: ${market.last_price:.2f} | Volume: {market.volume_24h}")
                print(f"     Spread: Yes({market.yes_bid:.2f}-{market.yes_ask:.2f}) No({market.no_bid:.2f}-{market.no_ask:.2f})")
        print("="*50)

def main():
    print("Kalshi Arbitrage Opportunity Finder")
    print("="*50)
    openai_key = os.getenv("OPENAI_API_KEY")
    kalshi_key = os.getenv("KALSHI_API_KEY")
    if not openai_key: print("OPENAI_API_KEY required"); return 1
    if not kalshi_key: print("KALSHI_API_KEY recommended")
    try:
        finder = ArbitrageFinder(openai_key, kalshi_key)
        opportunities = finder.find_opportunities("2024 election markets and political events", 15, 20, True, 0.6)
        finder.print_opportunities(opportunities)
    except Exception as e: print(f"Error: {e}"); return 1
    return 0

if __name__ == "__main__": exit(main())