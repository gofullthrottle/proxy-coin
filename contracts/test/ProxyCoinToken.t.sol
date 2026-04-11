// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/ProxyCoinToken.sol";

contract ProxyCoinTokenTest is Test {
    ProxyCoinToken public token;
    address public admin = address(this);
    address public minter = address(0x1);
    address public burner = address(0x2);
    address public user = address(0x3);

    function setUp() public {
        token = new ProxyCoinToken(admin);
        token.grantRole(token.MINTER_ROLE(), minter);
        token.grantRole(token.BURNER_ROLE(), burner);
    }

    // -------------------------------------------------------------------------
    // Metadata
    // -------------------------------------------------------------------------

    function test_NameAndSymbol() public view {
        assertEq(token.name(), "Proxy Coin");
        assertEq(token.symbol(), "PRXY");
    }

    function test_MaxSupply() public view {
        assertEq(token.MAX_SUPPLY(), 1_000_000_000 * 1e18);
    }

    function test_Decimals() public view {
        assertEq(token.decimals(), 18);
    }

    // -------------------------------------------------------------------------
    // Minting
    // -------------------------------------------------------------------------

    function test_Mint() public {
        uint256 amount = 1_000 * 1e18;
        vm.prank(minter);
        token.mint(user, amount);
        assertEq(token.balanceOf(user), amount);
        assertEq(token.totalSupply(), amount);
    }

    function test_MintToZeroAddress() public {
        // ERC20 base reverts on mint to address(0)
        vm.prank(minter);
        vm.expectRevert();
        token.mint(address(0), 1e18);
    }

    function test_MintExceedsMaxSupply() public {
        uint256 maxSupply = token.MAX_SUPPLY();
        // Mint up to max supply successfully
        vm.prank(minter);
        token.mint(user, maxSupply);
        assertEq(token.totalSupply(), maxSupply);

        // One more wei should revert
        vm.prank(minter);
        vm.expectRevert("Exceeds max supply");
        token.mint(user, 1);
    }

    function test_MintExactlyMaxSupply() public {
        uint256 maxSupply = token.MAX_SUPPLY();
        vm.prank(minter);
        token.mint(user, maxSupply);
        assertEq(token.totalSupply(), maxSupply);
    }

    function test_MintUnauthorized() public {
        vm.prank(user);
        vm.expectRevert();
        token.mint(user, 1_000 * 1e18);
    }

    function test_MintMultipleTimes() public {
        vm.startPrank(minter);
        token.mint(user, 500 * 1e18);
        token.mint(user, 500 * 1e18);
        vm.stopPrank();
        assertEq(token.balanceOf(user), 1_000 * 1e18);
    }

    // -------------------------------------------------------------------------
    // Burning
    // -------------------------------------------------------------------------

    function test_Burn() public {
        uint256 mintAmount = 1_000 * 1e18;
        uint256 burnAmount = 400 * 1e18;

        // Mint to burner address so it holds tokens
        vm.prank(minter);
        token.mint(burner, mintAmount);
        assertEq(token.balanceOf(burner), mintAmount);

        // Burner burns from its own balance
        vm.prank(burner);
        token.burn(burnAmount);

        assertEq(token.balanceOf(burner), mintAmount - burnAmount);
        assertEq(token.totalSupply(), mintAmount - burnAmount);
    }

    function test_BurnUnauthorized() public {
        // Mint to user first
        vm.prank(minter);
        token.mint(user, 1_000 * 1e18);

        // user does not have BURNER_ROLE
        vm.prank(user);
        vm.expectRevert();
        token.burn(100 * 1e18);
    }

    function test_BurnReducesTotalSupply() public {
        uint256 amount = 500 * 1e18;
        vm.prank(minter);
        token.mint(burner, amount);

        vm.prank(burner);
        token.burn(amount);

        assertEq(token.totalSupply(), 0);
    }

    // -------------------------------------------------------------------------
    // Role Management
    // -------------------------------------------------------------------------

    function test_GrantRole() public {
        address newMinter = address(0x4);
        assertFalse(token.hasRole(token.MINTER_ROLE(), newMinter));

        token.grantRole(token.MINTER_ROLE(), newMinter);
        assertTrue(token.hasRole(token.MINTER_ROLE(), newMinter));

        // Verify new minter can actually mint
        vm.prank(newMinter);
        token.mint(user, 100 * 1e18);
        assertEq(token.balanceOf(user), 100 * 1e18);
    }

    function test_RevokeRole() public {
        assertTrue(token.hasRole(token.MINTER_ROLE(), minter));

        token.revokeRole(token.MINTER_ROLE(), minter);
        assertFalse(token.hasRole(token.MINTER_ROLE(), minter));

        // Revoked minter can no longer mint
        vm.prank(minter);
        vm.expectRevert();
        token.mint(user, 100 * 1e18);
    }

    function test_GrantRoleUnauthorized() public {
        // user cannot grant roles (not admin)
        vm.prank(user);
        vm.expectRevert();
        token.grantRole(token.MINTER_ROLE(), user);
    }

    function test_AdminHasDefaultAdminRole() public view {
        assertTrue(token.hasRole(token.DEFAULT_ADMIN_ROLE(), admin));
    }

    function test_MinterAndBurnerRolesSet() public view {
        assertTrue(token.hasRole(token.MINTER_ROLE(), minter));
        assertTrue(token.hasRole(token.BURNER_ROLE(), burner));
    }

    // -------------------------------------------------------------------------
    // Transfers
    // -------------------------------------------------------------------------

    function test_Transfer() public {
        uint256 amount = 1_000 * 1e18;
        vm.prank(minter);
        token.mint(user, amount);

        address recipient = address(0x5);
        vm.prank(user);
        token.transfer(recipient, 300 * 1e18);

        assertEq(token.balanceOf(user), 700 * 1e18);
        assertEq(token.balanceOf(recipient), 300 * 1e18);
    }

    function test_TransferInsufficientBalance() public {
        vm.prank(minter);
        token.mint(user, 100 * 1e18);

        vm.prank(user);
        vm.expectRevert();
        token.transfer(address(0x5), 200 * 1e18);
    }

    function test_ApproveAndTransferFrom() public {
        uint256 amount = 1_000 * 1e18;
        address spender = address(0x5);
        address recipient = address(0x6);

        vm.prank(minter);
        token.mint(user, amount);

        // user approves spender
        vm.prank(user);
        token.approve(spender, 500 * 1e18);
        assertEq(token.allowance(user, spender), 500 * 1e18);

        // spender transfers on behalf of user
        vm.prank(spender);
        token.transferFrom(user, recipient, 500 * 1e18);

        assertEq(token.balanceOf(user), 500 * 1e18);
        assertEq(token.balanceOf(recipient), 500 * 1e18);
        assertEq(token.allowance(user, spender), 0);
    }

    function test_TransferFromExceedsAllowance() public {
        vm.prank(minter);
        token.mint(user, 1_000 * 1e18);

        address spender = address(0x5);
        vm.prank(user);
        token.approve(spender, 100 * 1e18);

        vm.prank(spender);
        vm.expectRevert();
        token.transferFrom(user, address(0x6), 200 * 1e18);
    }

    // -------------------------------------------------------------------------
    // Edge cases
    // -------------------------------------------------------------------------

    function test_MintZeroAmount() public {
        // Minting 0 is allowed by ERC20 but produces no supply change
        uint256 supplyBefore = token.totalSupply();
        vm.prank(minter);
        token.mint(user, 0);
        assertEq(token.totalSupply(), supplyBefore);
    }

    function test_BurnMoreThanBalance() public {
        vm.prank(minter);
        token.mint(burner, 100 * 1e18);

        vm.prank(burner);
        vm.expectRevert();
        token.burn(200 * 1e18);
    }
}
