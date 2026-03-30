# fibr-gen

[日本語](README_ja.md) | [简体中文](README_zh.md)

**fibr-gen** is a flexible report generator based on Excel templates. Try [here](https://fibr-gen-production.up.railway.app/).

The name **fibr-gen** is derived from "**f**LEX**ib**LE **r**EPORT **gen**ERATOR".

## Introduction

This system generates Excel reports based on specified templates and data configurations. It abstracts data sources into **Data Views** and maps them to Excel cells using **Blocks**.

## Features

- **Template-Based**: Design your reports using standard Excel (`.xlsx`) files.
- **Data Views**: Decouple data retrieval from report layout using SQL-based or memory-based views.
- **Flexible Blocks**:
  - **ValueBlock** (type: `value`): Simple data filling (lists, single values).
  - **MatrixBlock** (type: `matrix`): Complex layouts like cross-tabs (pivot tables) with dynamic expansion in both vertical and horizontal directions.
  - **HeaderBlock** (type: `header`): Defines headers or axes for matrix layouts.
- **Dynamic Sheets**: Generate multiple sheets dynamically based on data (e.g., one sheet per department or month).

## Documentation

- [Configuration Guide & Tutorial](docs/guide_en.md)

## Getting Started

*(Currently under active development)*

To run the core tests and see the generator in action:

```bash
go test -v ./core
```

## License

MIT
