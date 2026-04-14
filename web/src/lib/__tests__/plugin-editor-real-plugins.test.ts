/**
 * Integration tests that verify real-world plugin YAML structures can be
 * correctly parsed through stateFromDraft → buildDraft roundtrip.
 *
 * These structures mirror the actual plugin YAML files in /yamdc-plugin/plugins/.
 * If any plugin fails to roundtrip, the refactored editor cannot safely import
 * and re-export that plugin's configuration.
 */
import { describe, expect, it } from "vitest";

import type { PluginEditorDraft } from "@/lib/api";
import {
  buildDraft,
  stateFromDraft,
} from "@/components/plugin-editor/plugin-editor-utils";

// Helper: round-trip a draft through stateFromDraft → buildDraft
// and verify key structural invariants.
function roundtripDraft(draft: PluginEditorDraft) {
  const state = stateFromDraft(draft);
  const rebuilt = buildDraft(state);
  return { state, rebuilt };
}

// ---------------------------------------------------------------------------
// one-step plugins
// ---------------------------------------------------------------------------

describe("real plugin: alpha_site (one-step, headers, cookies, postprocess.defaults)", () => {
  const draft: PluginEditorDraft = {
    version: 1,
    name: "alpha_site",
    type: "one-step",
    hosts: ["https://alpha.example.com"],
    request: {
      method: "GET",
      path: "/${number}",
      headers: {
        Accept: "text/html,application/xhtml+xml",
        "Accept-Language": "en-US,en;q=0.5",
      },
      cookies: { existmag: "mag", age: "verified", dv: "1" },
      accept_status_codes: [200],
      not_found_status_codes: [404],
    },
    scrape: {
      format: "html",
      fields: {
        number: { selector: { kind: "xpath", expr: "//span[2]/text()" }, parser: "string", required: true },
        title: { selector: { kind: "xpath", expr: "//h3" }, parser: "string", required: true },
        actors: { selector: { kind: "xpath", expr: "//a/text()", multi: true }, transforms: [{ kind: "remove_empty" }, { kind: "dedupe" }], parser: "string_list" },
        cover: { selector: { kind: "xpath", expr: "//a/@href" }, parser: "string", required: true },
      },
    },
    postprocess: {
      defaults: { title_lang: "ja" },
    },
  };

  it("imports correctly", () => {
    const { state } = roundtripDraft(draft);
    expect(state.name).toBe("alpha_site");
    expect(state.type).toBe("one-step");
    expect(state.hostsText).toBe("https://alpha.example.com");
    expect(state.request.method).toBe("GET");
    expect(state.request.path).toBe("/${number}");
    expect(JSON.parse(state.request.headersJSON)).toHaveProperty("Accept");
    expect(JSON.parse(state.request.cookiesJSON)).toEqual({ existmag: "mag", age: "verified", dv: "1" });
    expect(state.request.notFoundStatusText).toBe("404");
    expect(state.postTitleLang).toBe("ja");
    expect(state.multiRequestEnabled).toBe(false);
    expect(state.workflowEnabled).toBe(false);
  });

  it("roundtrips preserving structure", () => {
    const { rebuilt } = roundtripDraft(draft);
    expect(rebuilt.name).toBe("alpha_site");
    expect(rebuilt.type).toBe("one-step");
    expect(rebuilt.hosts).toEqual(["https://alpha.example.com"]);
    expect(rebuilt.request?.method).toBe("GET");
    expect(rebuilt.request?.path).toBe("/${number}");
    expect(rebuilt.request?.headers).toHaveProperty("Accept");
    expect(rebuilt.request?.cookies).toEqual({ existmag: "mag", age: "verified", dv: "1" });
    expect(rebuilt.request?.not_found_status_codes).toEqual([404]);
    expect(rebuilt.postprocess?.defaults?.title_lang).toBe("ja");
    expect(rebuilt.multi_request).toBeUndefined();
    expect(rebuilt.workflow).toBeUndefined();
  });
});

describe("real plugin: archive_press (one-step, decode_charset, parser with layout, postprocess.assign)", () => {
  const draft: PluginEditorDraft = {
    version: 1,
    name: "archive_press",
    type: "one-step",
    hosts: ["https://archive.example.com"],
    request: {
      method: "GET",
      path: "/moviepages/${number}/index.html",
      accept_status_codes: [200],
      not_found_status_codes: [404],
      response: { decode_charset: "euc-jp" },
    },
    scrape: {
      format: "html",
      fields: {
        title: { selector: { kind: "xpath", expr: "//h1/text()" }, parser: "string", required: true },
        release_date: {
          selector: { kind: "xpath", expr: "//span[@class='spec-content']/text()" },
          parser: { kind: "date_layout_soft", layout: "2006-01-02" },
        },
        duration: { selector: { kind: "xpath", expr: "//span[@class='spec-content']/text()" }, parser: "duration_hhmmss" },
      },
    },
    postprocess: {
      assign: { number: "${number}", cover: '${concat(${host}, "/moviepages/", ${number}, "/images/l_l.jpg")}' },
      defaults: { title_lang: "ja", plot_lang: "ja" },
    },
  };

  it("imports decode_charset and parser with layout", () => {
    const { state } = roundtripDraft(draft);
    expect(state.request.decodeCharset).toBe("euc-jp");
    // release_date should have parser kind and layout set
    const releaseDateField = state.fields.find((f) => f.name === "release_date");
    expect(releaseDateField?.parserKind).toBe("date_layout_soft");
    expect(releaseDateField?.parserLayout).toBe("2006-01-02");
    // postprocess assign
    expect(state.postAssign).toHaveLength(2);
    expect(state.postAssign.find((a) => a.key === "number")?.value).toBe("${number}");
    expect(state.postTitleLang).toBe("ja");
    expect(state.postPlotLang).toBe("ja");
  });

  it("roundtrips preserving decode_charset and parser layout", () => {
    const { rebuilt } = roundtripDraft(draft);
    expect(rebuilt.request?.response?.decode_charset).toBe("euc-jp");
    expect(rebuilt.postprocess?.assign?.number).toBe("${number}");
    const rdField = rebuilt.scrape.fields.release_date;
    expect(rdField).toBeDefined();
    const rdParser = rdField.parser as { kind: string; layout: string };
    expect(rdParser.kind).toBe("date_layout_soft");
    expect(rdParser.layout).toBe("2006-01-02");
  });
});

describe("real plugin: cospuri (one-step, switch_config, regex_extract, precheck patterns)", () => {
  const draft: PluginEditorDraft = {
    version: 1,
    name: "cospuri",
    type: "one-step",
    hosts: ["https://www.cospuri.com"],
    precheck: {
      number_patterns: ["^(?i)COSPURI-[0-9]{4}[A-Za-z0-9]{4}$"],
    },
    request: {
      method: "GET",
      path: "/sample",
      query: { id: '${to_lower(${trim_prefix(${number}, "COSPURI-")})}' },
      accept_status_codes: [200],
      not_found_status_codes: [404],
    },
    scrape: {
      format: "html",
      fields: {
        title: { selector: { kind: "xpath", expr: "//div[@class='description']/text()" }, parser: "string", required: true },
        cover: {
          selector: { kind: "xpath", expr: "//div/@style" },
          transforms: [{ kind: "regex_extract", value: "(?i)url\\((.*)\\s*\\)", index: 1 }],
          parser: "string",
          required: true,
        },
      },
    },
    postprocess: {
      defaults: { title_lang: "en", plot_lang: "en", genres_lang: "en" },
      switch_config: { disable_release_date_check: true },
    },
  };

  it("imports switch_config, precheck patterns, and regex_extract", () => {
    const { state } = roundtripDraft(draft);
    expect(state.precheckPatternsText).toBe("^(?i)COSPURI-[0-9]{4}[A-Za-z0-9]{4}$");
    expect(state.postDisableReleaseDateCheck).toBe(true);
    expect(state.postGenresLang).toBe("en");
    // regex_extract transform
    const coverField = state.fields.find((f) => f.name === "cover");
    expect(coverField).toBeDefined();
    expect(coverField?.transforms).toHaveLength(1);
    expect(coverField?.transforms[0].kind).toBe("regex_extract");
    expect(coverField?.transforms[0].value).toBe("(?i)url\\((.*)\\s*\\)");
    expect(coverField?.transforms[0].index).toBe("1");
  });

  it("roundtrips preserving switch_config", () => {
    const { rebuilt } = roundtripDraft(draft);
    expect(rebuilt.postprocess?.switch_config?.disable_release_date_check).toBe(true);
    expect(rebuilt.precheck?.number_patterns).toEqual(["^(?i)COSPURI-[0-9]{4}[A-Za-z0-9]{4}$"]);
    // regex_extract roundtrip
    const coverTransforms = rebuilt.scrape.fields.cover?.transforms;
    expect(coverTransforms).toHaveLength(1);
    expect(coverTransforms?.[0].kind).toBe("regex_extract");
    expect(coverTransforms?.[0].value).toBe("(?i)url\\((.*)\\s*\\)");
    expect(coverTransforms?.[0].index).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// two-step (workflow) plugins
// ---------------------------------------------------------------------------

describe("real plugin: movie_hub (two-step, precheck.variables, workflow, postprocess.defaults)", () => {
  const draft: PluginEditorDraft = {
    version: 1,
    name: "movie_hub",
    type: "two-step",
    hosts: ["https://movies.example.com"],
    precheck: {
      variables: { clean_number: "${clean_number(${number})}" },
    },
    request: {
      method: "GET",
      path: "/search",
      query: { q: "${number}", f: "all" },
      accept_status_codes: [200],
    },
    workflow: {
      search_select: {
        selectors: [
          { name: "read_link", kind: "xpath", expr: '//div[@class="movie-list h cols-4 vcols-8"]/div[@class="item"]/a/@href' },
          { name: "read_number", kind: "xpath", expr: '//div[@class="movie-list h cols-4 vcols-8"]/div[@class="item"]/a/div[@class="video-title"]/strong' },
        ],
        item_variables: { clean_item_number: "${clean_number(${item.read_number})}" },
        match: {
          mode: "and",
          conditions: ['equals("${vars.clean_number}", "${item_variables.clean_item_number}")'],
        },
        return: "${item.read_link}",
        next_request: {
          method: "GET",
          path: "${value}",
          accept_status_codes: [200],
        },
      },
    },
    scrape: {
      format: "html",
      fields: {
        number: { selector: { kind: "xpath", expr: "//a/@data-clipboard-text" }, parser: "string", required: true },
        title: { selector: { kind: "xpath", expr: "//strong" }, parser: "string", required: true },
        cover: { selector: { kind: "xpath", expr: "//a/img/@src" }, parser: "string", required: true },
      },
    },
    postprocess: {
      defaults: { title_lang: "ja" },
    },
  };

  it("imports all workflow details", () => {
    const { state } = roundtripDraft(draft);
    expect(state.workflowEnabled).toBe(true);
    expect(state.workflowSelectors).toHaveLength(2);
    expect(state.workflowSelectors[0].name).toBe("read_link");
    expect(state.workflowSelectors[1].name).toBe("read_number");
    expect(state.workflowItemVariables).toHaveLength(1);
    expect(state.workflowItemVariables[0].key).toBe("clean_item_number");
    expect(state.workflowMatchMode).toBe("and");
    expect(state.workflowReturn).toBe("${item.read_link}");
    expect(state.workflowNextRequest.method).toBe("GET");
    expect(state.workflowNextRequest.path).toBe("${value}");
    // precheck variables
    expect(state.precheckVariables).toHaveLength(1);
    expect(state.precheckVariables[0].key).toBe("clean_number");
    // request query
    expect(JSON.parse(state.request.queryJSON)).toEqual({ q: "${number}", f: "all" });
  });

  it("roundtrips preserving workflow and precheck variables", () => {
    const { rebuilt } = roundtripDraft(draft);
    expect(rebuilt.workflow?.search_select?.selectors).toHaveLength(2);
    expect(rebuilt.workflow?.search_select?.item_variables).toEqual({ clean_item_number: "${clean_number(${item.read_number})}" });
    expect(rebuilt.workflow?.search_select?.match?.mode).toBe("and");
    expect(rebuilt.workflow?.search_select?.return).toBe("${item.read_link}");
    expect(rebuilt.workflow?.search_select?.next_request?.path).toBe("${value}");
    expect(rebuilt.precheck?.variables).toEqual({ clean_number: "${clean_number(${number})}" });
  });
});

describe("real plugin: missav (two-step, next_request with url and headers)", () => {
  const draft: PluginEditorDraft = {
    version: 1,
    name: "missav",
    type: "two-step",
    hosts: ["https://missav.ws"],
    request: {
      method: "GET",
      path: "/cn/search/${number}",
      headers: { "Sec-GPC": "1" },
      accept_status_codes: [200],
    },
    workflow: {
      search_select: {
        selectors: [
          { name: "read_link", kind: "xpath", expr: "//a/@href" },
          { name: "read_title", kind: "xpath", expr: "//a/text()" },
        ],
        match: {
          mode: "and",
          conditions: ['contains("${item.read_title}", "${number}")'],
        },
        return: "${item.read_link}",
        next_request: {
          method: "GET",
          url: "${build_url(${host}, ${value})}",
          headers: { "Sec-GPC": "1" },
          accept_status_codes: [200],
        },
      },
    },
    scrape: {
      format: "html",
      fields: {
        number: { selector: { kind: "xpath", expr: "//span/text()" }, parser: "string", required: true },
        title: { selector: { kind: "xpath", expr: "//h1/text()" }, parser: "string", required: true },
        cover: { selector: { kind: "xpath", expr: "//link/@href" }, parser: "string", required: true },
      },
    },
  };

  it("imports next_request with url (not path) and headers", () => {
    const { state } = roundtripDraft(draft);
    expect(state.workflowNextRequest.rawURL).toBe("${build_url(${host}, ${value})}");
    expect(state.workflowNextRequest.path).toBe("");
    expect(JSON.parse(state.workflowNextRequest.headersJSON)).toEqual({ "Sec-GPC": "1" });
    // primary request headers
    expect(JSON.parse(state.request.headersJSON)).toEqual({ "Sec-GPC": "1" });
  });

  it("roundtrips preserving url vs path distinction", () => {
    const { rebuilt } = roundtripDraft(draft);
    expect(rebuilt.workflow?.search_select?.next_request?.url).toBe("${build_url(${host}, ${value})}");
    expect(rebuilt.workflow?.search_select?.next_request?.path).toBeUndefined();
    expect(rebuilt.workflow?.search_select?.next_request?.headers).toEqual({ "Sec-GPC": "1" });
  });
});

// ---------------------------------------------------------------------------
// multi_request + workflow plugin
// ---------------------------------------------------------------------------

describe("real plugin: avsox (multi_request, workflow, expect_count, next_request with url)", () => {
  const draft: PluginEditorDraft = {
    version: 1,
    name: "avsox",
    type: "two-step",
    hosts: ["https://avsox.click"],
    multi_request: {
      unique: true,
      candidates: [
        "${to_upper(${number})}",
        '${replace(${to_upper(${number})}, "-", "_")}',
        '${replace(${replace(${to_upper(${number})}, "-", "_")}, "_", "")}',
      ],
      request: {
        method: "GET",
        path: "/cn/search/${candidate}",
        accept_status_codes: [200],
      },
      success_when: {
        mode: "and",
        conditions: ['selector_exists(xpath("//*[@id=\'waterfall\']/div/a"))'],
      },
    },
    workflow: {
      search_select: {
        selectors: [{ name: "read_link", kind: "xpath", expr: '//*[@id="waterfall"]/div/a/@href' }],
        match: {
          mode: "and",
          conditions: ['contains("${item.read_link}", "movie")'],
          expect_count: 1,
        },
        return: "${item.read_link}",
        next_request: {
          method: "GET",
          url: '${build_url("https:", ${value})}',
          accept_status_codes: [200],
        },
      },
    },
    scrape: {
      format: "html",
      fields: {
        number: { selector: { kind: "xpath", expr: "//span[2]/text()" }, parser: "string", required: true },
        title: { selector: { kind: "xpath", expr: "//h3/text()" }, parser: "string", required: true },
        cover: { selector: { kind: "xpath", expr: "//img/@src" }, parser: "string", required: true },
      },
    },
    postprocess: {
      defaults: { title_lang: "ja" },
    },
  };

  it("imports multi_request with candidates and success_when", () => {
    const { state } = roundtripDraft(draft);
    expect(state.multiRequestEnabled).toBe(true);
    expect(state.request.method).toBe("GET");
    expect(state.request.path).toBe("/cn/search/${candidate}");
    expect(state.multiCandidatesText.split("\n")).toHaveLength(3);
    expect(state.multiSuccessMode).toBe("and");
    expect(state.multiSuccessConditionsText).toContain("selector_exists");
  });

  it("imports workflow with expect_count", () => {
    const { state } = roundtripDraft(draft);
    expect(state.workflowEnabled).toBe(true);
    expect(state.workflowExpectCountText).toBe("1");
    expect(state.workflowNextRequest.rawURL).toBe('${build_url("https:", ${value})}');
    expect(state.workflowNextRequest.path).toBe("");
  });

  it("roundtrips preserving multi_request and expect_count", () => {
    const { rebuilt } = roundtripDraft(draft);
    // multi_request
    expect(rebuilt.request).toBeNull();
    expect(rebuilt.multi_request).toBeDefined();
    expect(rebuilt.multi_request?.candidates).toHaveLength(3);
    expect(rebuilt.multi_request?.request?.path).toBe("/cn/search/${candidate}");
    expect(rebuilt.multi_request?.success_when?.mode).toBe("and");
    // workflow
    expect(rebuilt.workflow?.search_select?.match?.expect_count).toBe(1);
    expect(rebuilt.workflow?.search_select?.next_request?.url).toBe('${build_url("https:", ${value})}');
    expect(rebuilt.workflow?.search_select?.next_request?.path).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// source_a: split_index transform + parser with layout + postprocess assign
// ---------------------------------------------------------------------------

describe("real plugin: source_a (precheck patterns, split_index, date_layout_soft, postprocess.assign)", () => {
  const draft: PluginEditorDraft = {
    version: 1,
    name: "source_a",
    type: "one-step",
    hosts: ["https://source-a.example.com"],
    precheck: {
      number_patterns: ["^(?i)source_a[-_]?.+$"],
    },
    request: {
      method: "GET",
      path: '/${last_segment(${number}, "-")}/',
      accept_status_codes: [200],
      not_found_status_codes: [404],
    },
    scrape: {
      format: "html",
      fields: {
        title: { selector: { kind: "xpath", expr: "//title/text()" }, parser: "string", required: true },
        release_date: {
          selector: { kind: "xpath", expr: "//div[2]/p/text()" },
          transforms: [
            { kind: "split_index", sep: ":", index: 1 },
            { kind: "trim" },
          ],
          parser: { kind: "date_layout_soft", layout: "2006/01/02" },
        },
        cover: { selector: { kind: "xpath", expr: "//img/@src" }, parser: "string", required: true },
      },
    },
    postprocess: {
      assign: { number: "${number}" },
      defaults: { title_lang: "ja" },
    },
  };

  it("imports split_index transform and parser with layout", () => {
    const { state } = roundtripDraft(draft);
    const rdField = state.fields.find((f) => f.name === "release_date");
    expect(rdField).toBeDefined();
    expect(rdField?.transforms).toHaveLength(2);
    expect(rdField?.transforms[0].kind).toBe("split_index");
    expect(rdField?.transforms[0].sep).toBe(":");
    expect(rdField?.transforms[0].index).toBe("1");
    expect(rdField?.transforms[1].kind).toBe("trim");
    expect(rdField?.parserKind).toBe("date_layout_soft");
    expect(rdField?.parserLayout).toBe("2006/01/02");
  });

  it("roundtrips preserving transforms and parser layout", () => {
    const { rebuilt } = roundtripDraft(draft);
    const rdField = rebuilt.scrape.fields.release_date;
    expect(rdField.transforms).toHaveLength(2);
    expect(rdField.transforms?.[0].kind).toBe("split_index");
    expect(rdField.transforms?.[0].sep).toBe(":");
    expect(rdField.transforms?.[0].index).toBe(1);
    expect(rdField.transforms?.[1].kind).toBe("trim");
    const parser = rdField.parser as { kind: string; layout: string };
    expect(parser.kind).toBe("date_layout_soft");
    expect(parser.layout).toBe("2006/01/02");
  });
});

// ---------------------------------------------------------------------------
// 18av: trim_prefix + replace transforms
// ---------------------------------------------------------------------------

describe("real plugin: 18av (two-step, trim_prefix, replace transforms, next_request.url)", () => {
  const draft: PluginEditorDraft = {
    version: 1,
    name: "18av",
    type: "two-step",
    hosts: ["https://18av.me"],
    request: {
      method: "GET",
      path: "/cn/search.php",
      query: { kw_type: "key", kw: "${number}" },
      accept_status_codes: [200],
    },
    workflow: {
      search_select: {
        selectors: [
          { name: "read_link", kind: "xpath", expr: "//a/@href" },
          { name: "read_title", kind: "xpath", expr: "//a/text()" },
        ],
        match: {
          mode: "and",
          conditions: ['contains("${to_upper(${item.read_title})}", "${to_upper(${number})}")'],
        },
        return: "${item.read_link}",
        next_request: {
          method: "GET",
          url: '${concat(${host}, "/cn", ${value})}',
          accept_status_codes: [200],
        },
      },
    },
    scrape: {
      format: "html",
      fields: {
        number: { selector: { kind: "xpath", expr: "//div[@class='number']/text()" }, parser: "string", required: true },
        title: { selector: { kind: "xpath", expr: "//h1/text()" }, parser: "string", required: true },
        plot: {
          selector: { kind: "xpath", expr: "//p/text()" },
          transforms: [
            { kind: "trim_prefix", value: "简介：" },
            { kind: "trim" },
          ],
          parser: "string",
        },
        cover: {
          selector: { kind: "xpath", expr: "//meta/@content" },
          transforms: [
            { kind: "replace", old: " ", new: "" },
          ],
          parser: "string",
          required: true,
        },
      },
    },
  };

  it("imports trim_prefix and replace transforms", () => {
    const { state } = roundtripDraft(draft);
    const plotField = state.fields.find((f) => f.name === "plot");
    expect(plotField?.transforms).toHaveLength(2);
    expect(plotField?.transforms[0].kind).toBe("trim_prefix");
    expect(plotField?.transforms[0].value).toBe("简介：");
    const coverField = state.fields.find((f) => f.name === "cover");
    expect(coverField?.transforms).toHaveLength(1);
    expect(coverField?.transforms[0].kind).toBe("replace");
    expect(coverField?.transforms[0].old).toBe(" ");
    expect(coverField?.transforms[0].newValue).toBe("");
  });

  it("roundtrips preserving transforms", () => {
    const { rebuilt } = roundtripDraft(draft);
    const plotTransforms = rebuilt.scrape.fields.plot?.transforms;
    expect(plotTransforms).toHaveLength(2);
    expect(plotTransforms?.[0].kind).toBe("trim_prefix");
    expect(plotTransforms?.[0].value).toBe("简介：");
    const coverTransforms = rebuilt.scrape.fields.cover?.transforms;
    expect(coverTransforms).toHaveLength(1);
    expect(coverTransforms?.[0].kind).toBe("replace");
    expect(coverTransforms?.[0].old).toBe(" ");
    expect(coverTransforms?.[0].new).toBe("");
  });
});
