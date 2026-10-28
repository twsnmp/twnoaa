# twnoaa 📡🛰️

**twnoaa** は、気象衛星（NOAA 15/18/19、METEOR-M2シリーズ）や宇宙通信（SSTV/ISS）の軌道計算・通過予測、RTL-SDRによる自動受信、リアルタイムDSP復調、衛星画像デコード、およびマルチモーダルAIによる気象画像解析を統合したオールインワンのデスクトップアプリケーションです。[Wails](https://wails.io/) と [Svelte 5](https://svelte.dev/)、Go で構築されており、TWSNMPシリーズプロジェクトの一つです。

> 🇬🇧 [English README](README.md)

---

## ✨ 主な特徴

- 🛰️ **自動軌道計算 & 通過予測** — 内蔵SGP4軌道計算エンジンにより、CelesTrakから最新TLEを自動取得して衛星の可視通過（パス）を正確に予測・自動スケジューリング
- 📻 **RTL-SDR 直接制御 & リアルタイムDSP** — RTL-SDR（`librtlsdr`）を直接制御し、Bias-Tee給電やゲイン調整、リアルタイムのFFTスペクトラム＆ウォーターフォール表示に対応
- 🧪 **内蔵シミュレーションモード** — RTL-SDRハードウェアが手元にない場合でも、模擬信号生成により受信・復調・AI解析の全ワークフローをすぐに体験可能
- 🖼️ **多彩な衛星デコーダー対応**
  - **NOAA APT**: FM復調・2400Hz副搬送波検波・フレーム同期相関処理による可視光/赤外線画像合成
  - **METEOR-M2 LRPT**: QPSK復調、Viterbi＆Reed-Solomon誤り訂正による高精細マルチスペクトルデジタル衛星画像
  - **SSTV**: ISS（国際宇宙ステーション）やアマチュア無線からの宇宙画像デコード（Martin, Scottie等）
- 🤖 **AI気象解析 & 猫アニメーション** — 復調された衛星画像をGemini、OpenAI、Anthropic、ローカルOllamaなどのマルチモーダルAIに連携し、前線・低気圧渦・雲パターンの自動気象分析レポートを生成（思考中の猫アニメーションつき）
- 📼 **Rawデータ（I/Q信号）録音 & リプレイ** — 受信した生信号をローカルに保存し、後から何度でもリプレイして再デコードやアルゴリズム検証が可能
- 💾 **完全ローカル永続化** — 組み込みKey-Valueストア（bbolt）により、受信履歴、衛星画像、AI解析レポートを完全ローカル管理
- 🌐 **二言語UI対応** — 日本語と英語の表示切替に対応

---

## 🔍 サポートする衛星・モード

| 衛星・通信 | 周波数 | 変調・モード | 解像度・特徴 |
|---|---|---|---|
| **NOAA 15** | 137.620 MHz | APT (FM / AM 2.4kHz Subcarrier) | 4 km/px, 可視光 & 熱赤外線 2チャンネル画像 |
| **NOAA 18** | 137.9125 MHz | APT (FM / AM 2.4kHz Subcarrier) | 4 km/px, 可視光 & 熱赤外線 2チャンネル画像 |
| **NOAA 19** | 137.100 MHz | APT (FM / AM 2.4kHz Subcarrier) | 4 km/px, 可視光 & 熱赤外線 2チャンネル画像 |
| **METEOR-M2-3/4** | 137.100 / 137.900 MHz | LRPT (72k/80k QPSK) | 1 km/px, デジタル高解像度マルチスペクトル画像 |
| **ISS / SSTV** | 145.800 MHz | SSTV (FM / 各種SSTVモード) | 国際宇宙ステーションからのイベント宇宙通信画像 |

---

## 🤖 AI連携による気象解析

衛星画像がデコードされると、受信メタデータ（衛星名、周波数、通過時刻、最大仰角、受信地座標）とともにマルチモーダルLLMへ自動送信され、以下のような分析が自動生成されます：

- **気象パターンの識別**: 温帯低気圧の渦、寒冷前線・温暖前線、台風の目、積乱雲群の特定
- **地形・ランドマークの認識**: 海岸線、島嶼、半島、雪氷域、海流境界の検出
- **日本語による専門的考察**: アマチュア衛星観測者向けの分かりやすく詳細な気象レポート

### 対応AIプロバイダー
- **Google Gemini**（`gemini-2.5-flash`, `gemini-1.5-pro` など）
- **OpenAI**（`gpt-4o`, `gpt-4o-mini` など）
- **Anthropic Claude**（`claude-3-5-sonnet` など）
- **Ollama**（ローカル動作: `llava`, `minicpm-v` など）

---

## 🛠️ 技術スタック

| コンポーネント | 採用技術 |
|---|---|
| **デスクトップ基盤** | [Wails v2](https://wails.io/) |
| **バックエンド** | Go 1.27 + CGO (`librtlsdr`) |
| **フロントエンド** | Svelte 5 (Runes) + TypeScript + Tailwind CSS |
| **データベース** | [bbolt](https://github.com/etcd-io/bbolt)（組み込みKey-Valueストア） |
| **SDR / DSP** | `github.com/jpoirier/gortlsdr`, ピュアGo DSPエンジン（WFM, QPSK, FFT, I/Q） |
| **軌道計算** | ピュアGo SGP4 伝搬モデル + CelesTrak TLE パーサー |
| **AIオーケストレーション** | `langchaingo` + 各種LLM APIクライアント |
| **パッケージング** | macOS PKG (Apple Notarization 公証対応), Windows ZIP, Linux tar.gz |

---

## 🚀 インストールと起動

### 配布パッケージの利用

[Releases](https://github.com/twsnmp/twnoaa/releases) より、お使いのOSに合ったパッケージをダウンロードしてください。

- **macOS (Apple Silicon)**: `twnoaa-v0.1.0-darwin-arm64.pkg` をダウンロードし、インストーラーを実行してください（Apple公証済み）。
- **Windows (x64)**: `twnoaa-windows-amd64.zip` をダウンロードし、任意のフォルダに展開して `twnoaa.exe` を実行してください（必要な `rtlsdr.dll` 同梱）。
- **Linux (x64)**: `twnoaa-linux-amd64.tar.gz` を展開して実行してください（要 `librtlsdr0`, `libwebkit2gtk-4.1-0`）。

---

## 💻 開発環境のセットアップ

### 前提条件

- [mise](https://mise.jdx.dev/)（Go、Node.js の管理）
- [Wails CLI v2.15+](https://wails.io/docs/gettingstarted/installation)
- RTL-SDR ライブラリ (`librtlsdr`):
  - macOS: `brew install librtlsdr`
  - Ubuntu/Debian: `sudo apt-get install -y librtlsdr-dev libgtk-3-dev libwebkit2gtk-4.1-dev`
  - Windows: prebuilt 64bit DLL & headers

### 依存関係のセットアップ

```bash
# ツールチェーンのインストール
mise install

# フロントエンドの依存関係インストール
cd frontend
npm install
cd ..
```

### 開発モード起動 (ホットリロード)

```bash
# 開発サーバー起動
mise run dev
# または
wails dev
```

### ビルド

```bash
# ローカルビルド
mise run build

# macOS 公証インストーラーパッケージ作成 (Developer ID & notarytool)
mise run release-mac
```

---

## 🗄️ データ保存場所

受信データ、ログ、衛星画像、および bbolt データベースファイルは以下のディレクトリに保存されます。

| OS | デフォルトパス |
|---|---|
| **macOS** | `~/Library/Application Support/twnoaa/` |
| **Windows** | `%APPDATA%\twnoaa\` |
| **Linux** | `~/.config/twnoaa/` |

---

## 📄 ライセンス

[Apache License 2.0](LICENSE)
