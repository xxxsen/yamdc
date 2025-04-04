resource
===

所有的水印图都使用相同大小, 800x400(圆角大小:180像素), 方便后续处理。

重建分辨率命令(尽量保持宽高比为2:1, 该命令不会填黑边): 

```shell
ffmpeg -i ./leak.png -s "800x400" leak_convert.png
```