# 字体来源与许可

Chatty 排版（Typst）使用的字体：

| 文件 | 来源 | 许可 |
|---|---|---|
| LibertinusSerif-*.otf | typst-assets（Libertinus）| OFL |
| NewCM10-*.otf / NewCMMath-*.otf | typst-assets（New Computer Modern）| OFL |
| DejaVuSansMono*.ttf | typst-assets | 自由许可（Bitstream Vera 派生）|
| InriaSerif-*.ttf | typst-dev-assets | OFL |
| Roboto-Regular.ttf | typst-dev-assets | Apache-2.0 |
| NotoColorEmoji-*.ttf | typst-dev-assets | OFL |

中文（Microsoft YaHei 雅黑）不再随包分发：应用运行时由后端从用户系统
`C:\Windows\Fonts\msyh.ttc / msyhbd.ttc` 代理读取（见 main.go sysFonts），
仅供本机排版使用、不参与再分发，遵循微软字体使用条款。

typst-assets / typst-dev-assets 内容许可见：
https://github.com/typst/typst-assets
