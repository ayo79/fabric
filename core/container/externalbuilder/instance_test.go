/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package externalbuilder_test

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/container/ccintf"
	"github.com/hyperledger/fabric/core/container/externalbuilder"
)

var _ = Describe("Instance", func() {
	var (
		logger   *flogging.FabricLogger
		instance *externalbuilder.Instance
	)

	BeforeEach(func() {
		enc := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{MessageKey: "msg"})
		core := zapcore.NewCore(enc, zapcore.AddSync(GinkgoWriter), zap.NewAtomicLevel())
		logger = flogging.NewFabricLogger(zap.New(core).Named("logger"))

		instance = &externalbuilder.Instance{
			PackageID: "test-ccid",
			Builder: &externalbuilder.Builder{
				Location: "testdata/goodbuilder",
				Logger:   logger,
			},
		}
	})

	Describe("Start", func() {
		It("invokes the builder's run command and sets the run status", func() {
			err := instance.Start(&ccintf.PeerConnection{
				Address: "fake-peer-address",
				TLSConfig: &ccintf.TLSConfig{
					ClientCert: []byte("fake-client-cert"),
					ClientKey:  []byte("fake-client-key"),
					RootCert:   []byte("fake-root-cert"),
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(instance.Session).NotTo(BeNil())

			errCh := make(chan error)
			go func() { errCh <- instance.Session.Wait() }()
			Eventually(errCh).Should(Receive(BeNil()))
		})
	})

	Describe("Stop", func() {
		It("terminates the process", func() {
			cmd := exec.Command("sleep", "90")
			sess, err := externalbuilder.Start(logger, cmd)
			Expect(err).NotTo(HaveOccurred())
			instance.Session = sess
			instance.TermTimeout = time.Minute

			errCh := make(chan error)
			go func() { errCh <- instance.Session.Wait() }()
			Consistently(errCh).ShouldNot(Receive())

			err = instance.Stop()
			Expect(err).ToNot(HaveOccurred())
			Eventually(errCh).Should(Receive(MatchError("signal: terminated")))
		})

		Context("when the process doesn't respond to SIGTERM within TermTimeout", func() {
			It("kills the process with malice", func() {
				cmd := exec.Command("testdata/ignoreterm.sh")
				sess, err := externalbuilder.Start(logger, cmd)
				Expect(err).NotTo(HaveOccurred())

				instance.Session = sess
				instance.TermTimeout = time.Second

				errCh := make(chan error)
				go func() { errCh <- instance.Session.Wait() }()
				Consistently(errCh).ShouldNot(Receive())

				err = instance.Stop()
				Expect(err).ToNot(HaveOccurred())
				Eventually(errCh).Should(Receive(MatchError("signal: killed")))
			})
		})

		Context("when the instance session has not been started", func() {
			It("returns an error", func() {
				instance.Session = nil
				err := instance.Stop()
				Expect(err).To(MatchError("instance has not been started"))
			})
		})
	})

	Describe("Wait", func() {
		BeforeEach(func() {
			err := instance.Start(&ccintf.PeerConnection{
				Address: "fake-peer-address",
				TLSConfig: &ccintf.TLSConfig{
					ClientCert: []byte("fake-client-cert"),
					ClientKey:  []byte("fake-client-key"),
					RootCert:   []byte("fake-root-cert"),
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the exit status of the run", func() {
			code, err := instance.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(0))
		})

		Context("when run exits with a non-zero status", func() {
			BeforeEach(func() {
				instance.Builder.Location = "testdata/failbuilder"
				instance.Builder.Name = "failbuilder"
				err := instance.Start(&ccintf.PeerConnection{
					Address: "fake-peer-address",
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the exit status of the run and accompanying error", func() {
				code, err := instance.Wait()
				Expect(err).To(MatchError("builder 'failbuilder' run failed: exit status 1"))
				Expect(code).To(Equal(1))
			})
		})

		Context("when the instance session has not been started", func() {
			It("returns an error", func() {
				instance.Session = nil
				exitCode, err := instance.Wait()
				Expect(err).To(MatchError("instance was not successfully started"))
				Expect(exitCode).To(Equal(-1))
			})
		})
	})
})
