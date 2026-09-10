class LyricsDisplay < Formula
  desc "Show real-time Apple Music lyrics in the macOS menu bar"
  homepage "https://github.com/AKAama/lyrics-display"
  url "https://github.com/AKAama/lyrics-display/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "5556bb3cb4125484a0df477fa04b81887f1b937bf203df797491fd3c468647eb"
  license "MIT"

  depends_on "go" => :build
  depends_on macos: :ventura

  def install
    ldflags = %W[
      -s
      -w
      -X main.version=#{version}
    ]

    system "go", "build", *std_go_args(ldflags: ldflags), "."
  end

  service do
    run [opt_bin/"lyrics-display", "--service"]
    keep_alive true
    log_path var/"log/lyrics-display.log"
    error_log_path var/"log/lyrics-display.log"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/lyrics-display --version")
  end
end
