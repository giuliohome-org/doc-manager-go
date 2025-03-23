-- Bootstrap Lazy.nvim
local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not vim.loop.fs_stat(lazypath) then
  vim.fn.system({
    "git", "clone", "--filter=blob:none",
    "https://github.com/folke/lazy.nvim.git", lazypath
  })
end
vim.opt.rtp:prepend(lazypath)

-- Load plugins using Lazy.nvim
require("lazy").setup({
  -- LSP support
  { "neovim/nvim-lspconfig" },
  { "williamboman/mason.nvim" },
  { "williamboman/mason-lspconfig.nvim" },

  -- nvim-treesitter for better syntax highlighting
  {
    'nvim-treesitter/nvim-treesitter',
    run = ':TSUpdate',
    config = function()
      require'nvim-treesitter.configs'.setup {
        ensure_installed = { 'go', 'rust' },  -- Add Go and Rust for Treesitter
        highlight = { enable = true },        -- Enable syntax highlighting
      }
    end
  },

  -- vim-go for Go language features
  {
    'fatih/vim-go',
    config = function()
      vim.g.go_def_mode = 'gopls'  -- Use gopls for Go definitions
    end
  },  

  -- Autocompletion
  { "hrsh7th/nvim-cmp", dependencies = {
    "hrsh7th/cmp-nvim-lsp",
    "hrsh7th/cmp-buffer",
    "hrsh7th/cmp-path",
    "hrsh7th/cmp-cmdline",
    "L3MON4D3/LuaSnip",
  }},
})

-- Setup Mason (LSP Manager)
require("mason").setup()
require("mason-lspconfig").setup({
  ensure_installed = { "gopls" }
})

-- Setup LSP for Go
local lspconfig = require("lspconfig")
lspconfig.gopls.setup{}

-- Setup completion
local cmp = require("cmp")
cmp.setup({
  mapping = cmp.mapping.preset.insert({
    ["<C-Space>"] = cmp.mapping.complete(), -- Trigger completion
    ["<CR>"] = cmp.mapping.confirm({ select = true }), -- Confirm selection
  }),
  sources = cmp.config.sources({
    { name = "nvim_lsp" }, -- Use LSP completion
    { name = "buffer" },
    { name = "path" },
  })
})

-- Keybindings for LSP
vim.api.nvim_create_autocmd("LspAttach", {
  callback = function(event)
    local opts = { buffer = event.buf }
    vim.keymap.set("n", "gd", vim.lsp.buf.definition, opts) -- Go to definition
    vim.keymap.set("n", "K", vim.lsp.buf.hover, opts) -- Show docs
    vim.keymap.set("n", "<leader>rn", vim.lsp.buf.rename, opts) -- Rename symbol
    vim.keymap.set("n", "<leader>ca", vim.lsp.buf.code_action, opts) -- Code actions
    vim.keymap.set("n", "gr", vim.lsp.buf.references, { noremap = true, silent = true })
    vim.keymap.set('n', '<leader>el', function()
      if #vim.diagnostic.get(0) == 0 then
        print("No errors 🎉")
	vim.defer_fn(function() vim.api.nvim_echo({{"", "Normal"}}, false, {}) end, 1500)  -- Clears after 1.5s
      else
        vim.diagnostic.setloclist()
      end
    end, { noremap = true, silent = true })
    vim.keymap.set('n', '<leader>ec', '<cmd>lclose<CR>', { noremap = true, silent = true })
  end,
})

