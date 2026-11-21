---
title: '示例：登记不是下葬'
description: '用一个文件的两次状态变化说明如何阅读账本。'
developmentDate: '2026-11-21'
translationKey: example
draft: true
---

给一个文件设定寿命后，它仍在原来的目录里。此时账本能回答「什么时候到期」，却不能回答「墓地里有什么」。只有实际下葬后，墓地里才有对应的内容可以复活。

区分这两个时刻，也就知道该验证什么：登记后核对期限和原位置；下葬后再核对墓地记录。参见[架构契约](https://github.com/woodfishhhh/obituary/blob/main/docs/architecture.md)。
